package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	"cloudque/pkg/config"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"cloudque/pkg/websocket"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/pkg/sftp"
)

// fileService 文件服务实现
type fileService struct {
	sessionManager    *ssh.SessionManager
	authService       AuthService
	redisRepo         repository.RedisRepository
	homeDirCache      sync.Map
	sessionCheckCache sync.Map
}

// NewFileService 创建文件服务
func NewFileService(sessionManager *ssh.SessionManager, authService AuthService, redisRepo repository.RedisRepository) FileService {
	return &fileService{
		sessionManager: sessionManager,
		authService:    authService,
		redisRepo:      redisRepo,
	}
}

// getOrReconnectSession 获取会话，如果断开则自动重连
func (s *fileService) getOrReconnectSession(userID int, isRoot bool) (*ssh.UserSession, error) {
	if s.sessionManager == nil {
		logger.Errorf("SSH会话管理器未初始化: userID=%d", userID)
		return nil, errors.New(errors.CodeInternalError, "SSH会话管理器未初始化")
	}

	// 1. 尝试获取现有会话
	session, err := s.sessionManager.GetSession(userID, isRoot)

	// 2.1 检查会话及客户端是否有效
	isValid := false
	if err == nil && session != nil && session.Client != nil {
		// 优化：增加会话检查缓存，避免频繁执行 echo 1
		needCheck := true
		if lastCheck, ok := s.sessionCheckCache.Load(userID); ok {
			if time.Since(lastCheck.(time.Time)) < 5*time.Second {
				needCheck = false
				isValid = true
			}
		}

		if needCheck {
			// 尝试轻量级 ping
			if _, pingErr := session.Client.ExecuteCommand("echo 1"); pingErr == nil {
				// 确保 SFTP 客户端也存在
				if session.Client.GetSFTPClient() != nil {
					isValid = true
					s.sessionCheckCache.Store(userID, time.Now())
				}
			}
		}
	}

	if isValid {
		return session, nil
	}

	// 2.2 会话无效或已断开，尝试通过 AuthService 恢复
	logger.Infof("SSH会话不存在或已断开，尝试恢复: userID=%d, isRoot=%t", userID, isRoot)

	if err := s.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
		logger.Errorf("恢复SSH会话失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, err
	}

	// 4. 重新获取会话
	session, err = s.sessionManager.GetSession(userID, isRoot)
	if err != nil {
		logger.Errorf("恢复后获取SSH会话失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, errors.New(errors.CodeUnauthorized, "SSH会话恢复失败")
	}

	// 5. 验证新会话
	if session == nil || session.Client == nil || session.Client.GetSFTPClient() == nil {
		logger.Errorf("SSH会话恢复后客户端不可用: userID=%d, isRoot=%t", userID, isRoot)
		return nil, errors.New(errors.CodeInternalError, "SSH客户端未连接")
	}

	return session, nil
}

// getSftpClient 获取用户的SFTP客户端
func (s *fileService) getSftpClient(userID int, isRoot bool) (*sftp.Client, error) {
	session, err := s.getOrReconnectSession(userID, isRoot)
	if err != nil {
		return nil, err
	}
	return session.Client.GetSFTPClient(), nil
}

// getSSHClient 获取用户的SSH客户端
func (s *fileService) getSSHClient(userID int, isRoot bool) (*server.Client, error) {
	session, err := s.getOrReconnectSession(userID, isRoot)
	if err != nil {
		return nil, err
	}
	return session.Client, nil
}

// executeSSHCommand 执行SSH命令
func (s *fileService) executeSSHCommand(userID int, cmd string, isRoot bool) (string, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("获取SSH客户端失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return "", err
	}

	output, err := client.ExecuteCommand(cmd)
	if err != nil {
		logger.Errorf("执行命令失败: userID=%d, cmd=%s, err=%v", userID, cmd, err)
		return "", errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, fmt.Sprintf("执行命令失败: %v", err), err)
	}

	return output, nil
}

// resolvePath 解析路径并应用权限隔离
func (s *fileService) resolvePath(userID int, p string, isRootMode bool) (string, error) {
	session, err := s.getOrReconnectSession(userID, isRootMode)
	if err != nil {
		logger.Errorf("获取SSH会话失败: userID=%d, isRootMode=%t, err=%v", userID, isRootMode, err)
		return "", err
	}
	//
	//管理员模式:只需要处理为空的情况，不需要验证路径直接返回
	if isRootMode {
		//防止所传path为空的情况
		if p == "" || p == "." {
			return "/", nil
		}
		return p, nil
	}

	// 用户模式：限制在 base_path/{username}
	basePath := config.Get().Server.BasePath
	if basePath == "" {
		basePath = "/home"
	}
	homeDir := path.Join(basePath, session.Username)

	logger.Infof("resolvePath debug: userID=%d, username=%s, basePath=%s, homeDir=%s, inputPath=%s",
		userID, session.Username, basePath, homeDir, p)

	// 检查并创建用户主目录，如果不存在
	// 优化：使用缓存避免每次都检查
	if _, checked := s.homeDirCache.Load(userID); !checked {
		sftpClient := session.Client.GetSFTPClient()
		_, err = sftpClient.Stat(homeDir)
		if err != nil && os.IsNotExist(err) {
			logger.Infof("用户主目录不存在，尝试创建: userID=%d, homeDir=%s", userID, homeDir)
			// 使用 sftpClient 或 sshClient 创建目录
			if err = session.Client.MkdirAll(homeDir); err != nil {
				logger.Errorf("创建用户主目录失败: userID=%d, homeDir=%s, err=%v", userID, homeDir, err)
				return "", errors.NewWithErr(errors.CodeInternalError, "创建用户主目录失败", err)
			}
			logger.Infof("用户主目录创建成功: userID=%d, homeDir=%s", userID, homeDir)
		} else if err != nil {
			logger.Errorf("检查用户主目录状态失败: userID=%d, homeDir=%s, err=%v", userID, homeDir, err)
			return "", errors.NewWithErr(errors.CodeInternalError, "检查用户主目录状态失败", err)
		}
		s.homeDirCache.Store(userID, true)
	}

	//防止所传path为空的情况
	if p == "" || p == "." {
		return homeDir, nil
	}

	// 关键修复：正确处理传入路径
	cleanPath := path.Clean(p)

	// 如果传入的是绝对路径 (如 "/home")
	if path.IsAbs(cleanPath) {
		// 1. 如果路径已经是用户的家目录或其子目录，直接使用
		if strings.HasPrefix(cleanPath, homeDir) {
			return cleanPath, nil
		}

		// 2. 如果路径是根目录 "/"，映射到家目录
		if cleanPath == "/" {
			return homeDir, nil
		}

		// 3. 如果路径试图访问家目录之外
		// 这里的逻辑应该是：
		// 任何不以 homeDir 开头的绝对路径，都应该被拒绝
		logger.Errorf("越权访问被拒绝: userID=%d, path=%s, homeDir=%s", userID, p, homeDir)
		return "", errors.New(errors.CodeForbidden, "访问被拒绝：只能访问用户主目录下的文件")
	}

	// 如果是相对路径，拼接到家目录下
	finalPath := path.Join(homeDir, cleanPath)

	// 二次确认，防止 ../ 逃逸
	if !strings.HasPrefix(finalPath, homeDir) {
		logger.Errorf("越权访问被拒绝: userID=%d, path=%s, finalPath=%s", userID, p, finalPath)
		return "", errors.New(errors.CodeForbidden, "越权访问被拒绝")
	}

	return finalPath, nil
}

// GetFileList 获取文件列表
func (s *fileService) GetFileList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error) {
	opUserID := userID

	logger.Info("GetFileList 被调用",
		zap.Int("user_id", userID),
		zap.Bool("isRootMode", isRootMode),
		zap.String("path", req.Path),
	)

	// 恢复上次访问的路径
	if req.Path == "" {
		ctx := context.Background()
		lastPathKey := fmt.Sprintf("file:last_path:%d", userID)
		if lastPath, err := s.redisRepo.Get(ctx, lastPathKey); err == nil && lastPath != "" {
			req.Path = lastPath
		}
	} else {
		// 保存当前访问的路径
		ctx := context.Background()
		lastPathKey := fmt.Sprintf("file:last_path:%d", userID)
		_ = s.redisRepo.Set(ctx, lastPathKey, req.Path, 7*24*time.Hour)
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	// 解析出权限正确的路径
	resolvedPath, err := s.resolvePath(opUserID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("解析路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", opUserID, req.Path, isRootMode, err)
		return nil, err
	}

	var dirs []*dto.DirectoryItem
	var files []*dto.FileItem
	var realPath string
	var cacheHit bool

	// 性能优化：尝试从缓存获取全量文件列表
	// 缓存Key只跟路径有关，跟搜索关键词无关（搜索在内存中进行）
	// 使用 resolvedPath 作为缓存 Key，确保路径唯一性
	listCacheKey := fmt.Sprintf("file:list:%d:%v:%s", opUserID, isRootMode, resolvedPath)

	if s.redisRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cachedDataStr, err := s.redisRepo.Get(ctx, listCacheKey)
		if err == nil && cachedDataStr != "" {
			var cachedData struct {
				RealPath string               `json:"realPath"`
				Dirs     []*dto.DirectoryItem `json:"dirs"`
				Files    []*dto.FileItem      `json:"files"`
			}
			if err := json.Unmarshal([]byte(cachedDataStr), &cachedData); err == nil {
				realPath = cachedData.RealPath
				dirs = cachedData.Dirs
				files = cachedData.Files
				cacheHit = true
				logger.Debugf("文件列表缓存命中: key=%s", listCacheKey)
			}
		}
	}

	if !cacheHit {
		// 启动对应的ftpclient
		client, err := s.getSftpClient(opUserID, isRootMode)
		if err != nil {
			logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", opUserID, isRootMode, err)
			return nil, err
		}

		//传化为真实可用的路径
		realPath, err = client.RealPath(resolvedPath)
		if err != nil {
			logger.Errorf("无法解析真实路径: userID=%d, path=%s, err=%v", opUserID, resolvedPath, err)
			return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("无法解析路径: %s", resolvedPath), err)
		}

		//读取目标目录下的所有文件和目录
		entries, err := client.ReadDir(realPath)
		if err != nil {
			logger.Errorf("读取目录失败: userID=%d, realPath=%s, err=%v", opUserID, realPath, err)
			return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("读取目录失败: %s", realPath), err)
		}

		// 性能优化：使用 Redis 缓存 UID -> Username 映射
		uidToName := make(map[uint32]string)
		cacheKey := "file:uid_map"

		// 1. 尝试从 Redis 获取缓存
		if s.redisRepo != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			cachedMap, err := s.redisRepo.Get(ctx, cacheKey)
			cancel()
			if err == nil && cachedMap != "" {
				pairs := strings.Split(cachedMap, ",")
				for _, pair := range pairs {
					kv := strings.Split(pair, ":")
					if len(kv) == 2 {
						if uid, err := strconv.ParseUint(kv[0], 10, 32); err == nil {
							uidToName[uint32(uid)] = kv[1]
						}
					}
				}
			}
		}

		// 2. 如果缓存为空或命中率低，则执行命令加载
		if len(uidToName) == 0 {
			// 限制：只有当条目数少于 2000 时才尝试解析用户名，否则大目录还是会有性能问题
			if len(entries) < 2000 {
				passwdOut, err := s.executeSSHCommand(opUserID, "getent passwd || cat /etc/passwd", isRootMode)
				if err != nil {
					logger.Warnf("获取用户列表失败(非致命): userID=%d, err=%v", opUserID, err)
				} else {
					var cachePairs []string
					lines := strings.Split(passwdOut, "\n")
					for _, line := range lines {
						parts := strings.Split(line, ":")
						if len(parts) >= 3 {
							username := parts[0]
							uidStr := parts[2]
							if uid, err := strconv.ParseUint(uidStr, 10, 32); err == nil {
								uidToName[uint32(uid)] = username
								cachePairs = append(cachePairs, fmt.Sprintf("%d:%s", uid, username))
							}
						}
					}

					// 3. 更新 Redis 缓存 (有效期 1 小时)
					if s.redisRepo != nil && len(cachePairs) > 0 {
						ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
						cacheVal := strings.Join(cachePairs, ",")
						_ = s.redisRepo.Set(ctx, cacheKey, cacheVal, 1*time.Hour)
						cancel()
					}
				}
			}
		}

		for _, entry := range entries {
			var owner string
			if sys := entry.Sys(); sys != nil {
				if stat, ok := sys.(*sftp.FileStat); ok {
					name, found := uidToName[stat.UID]
					if found {
						owner = name
					} else {
						owner = fmt.Sprintf("%d", stat.UID)
					}
				} else {
					owner = "unknown"
				}
			} else {
				owner = "unknown"
			}

			if entry.IsDir() {
				dirs = append(dirs, &dto.DirectoryItem{
					Name:      entry.Name(),
					UpdatedAt: entry.ModTime(),
					Path:      path.Join(realPath, entry.Name()),
					Owner:     owner,
				})
			} else {
				files = append(files, &dto.FileItem{
					Filename:  entry.Name(),
					FileSize:  entry.Size(),
					UpdatedAt: entry.ModTime(),
					Path:      path.Join(realPath, entry.Name()),
					Owner:     owner,
				})
			}
		}

		// 排序
		sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
		sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })

		// 存入缓存 (TTL 10秒)
		if s.redisRepo != nil {
			cacheData := struct {
				RealPath string               `json:"realPath"`
				Dirs     []*dto.DirectoryItem `json:"dirs"`
				Files    []*dto.FileItem      `json:"files"`
			}{
				RealPath: realPath,
				Dirs:     dirs,
				Files:    files,
			}
			if data, err := json.Marshal(cacheData); err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = s.redisRepo.Set(ctx, listCacheKey, string(data), 60*time.Second)
				cancel()
			}
		}
	}

	// 内存过滤 (Keyword)
	if req.Keyword != "" {
		var filteredDirs []*dto.DirectoryItem
		var filteredFiles []*dto.FileItem
		for _, d := range dirs {
			if strings.Contains(d.Name, req.Keyword) {
				filteredDirs = append(filteredDirs, d)
			}
		}
		for _, f := range files {
			if strings.Contains(f.Filename, req.Keyword) {
				filteredFiles = append(filteredFiles, f)
			}
		}
		dirs = filteredDirs
		files = filteredFiles
	}

	// Pagination
	totalDirs := len(dirs)
	totalFiles := len(files)
	total := totalDirs + totalFiles
	pages := total / req.PageSize
	if total%req.PageSize != 0 {
		pages++
	}

	start := (req.Page - 1) * req.PageSize

	var resultDirs []*dto.DirectoryItem
	var resultFiles []*dto.FileItem

	if start < total {
		currentIdx := 0
		count := 0

		// Add dirs
		for _, d := range dirs {
			if currentIdx >= start && count < req.PageSize {
				resultDirs = append(resultDirs, d)
				count++
			}
			currentIdx++
		}

		// Add files
		for _, f := range files {
			if currentIdx >= start && count < req.PageSize {
				resultFiles = append(resultFiles, f)
				count++
			}
			currentIdx++
		}
	} else {
		resultDirs = []*dto.DirectoryItem{}
		resultFiles = []*dto.FileItem{}
	}

	// Build Breadcrumb
	breadcrumb := buildBreadcrumb(realPath)

	return &dto.FilesListData{
		Breadcrumb:    breadcrumb,
		Path:          realPath,
		FileList:      resultFiles,
		DirectoryList: resultDirs,
		Total:         total,
		Pages:         pages,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}, nil
}

func buildBreadcrumb(p string) []*dto.BreadcrumbItem {
	parts := strings.Split(p, "/")
	var items []*dto.BreadcrumbItem

	// Always add Root for absolute paths
	if strings.HasPrefix(p, "/") {
		items = append(items, &dto.BreadcrumbItem{Name: "/", Path: "/"})
	}

	currentPath := ""
	for _, part := range parts {
		if part == "" {
			if currentPath == "" {
				currentPath = "/"
			}
			continue
		}
		if currentPath == "/" {
			currentPath += part
		} else {
			currentPath += "/" + part
		}
		items = append(items, &dto.BreadcrumbItem{
			Name: part,
			Path: currentPath,
		})
	}
	return items
}

// invalidateFileListCache 清除文件列表缓存
func (s *fileService) invalidateFileListCache(userID int, isRootMode bool, path string) {
	if s.redisRepo != nil {
		ctx := context.Background()
		key := fmt.Sprintf("file:list:%d:%v:%s", userID, isRootMode, path)
		_ = s.redisRepo.Del(ctx, key)
		logger.Debugf("清除文件列表缓存: key=%s", key)
	}
}

// UploadFile 上传文件
func (s *fileService) UploadFile(userID int, file multipart.File, header *multipart.FileHeader, targetPath string, isRootMode bool) (*dto.FileUploadData, error) {
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", userID, isRootMode, err)
		return nil, err
	}

	resolvedPath, err := s.resolvePath(userID, targetPath, isRootMode)
	if err != nil {
		logger.Errorf("解析目标路径失败: userID=%d, targetPath=%s, isRootMode=%t, err=%v", userID, targetPath, isRootMode, err)
		return nil, err
	}

	// Ensure target directory exists
	if err := client.MkdirAll(resolvedPath); err != nil {
		logger.Errorf("创建目录失败: userID=%d, resolvedPath=%s, err=%v", userID, resolvedPath, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("创建目录失败: %s", resolvedPath), err)
	}

	// 简单的文件名清理，防止路径遍历
	safeFilename := path.Base(header.Filename)
	remotePath := path.Join(resolvedPath, safeFilename)
	dst, err := client.Create(remotePath)
	if err != nil {
		logger.Errorf("创建远程文件失败: userID=%d, remotePath=%s, err=%v", userID, remotePath, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("创建远程文件失败: %s", remotePath), err)
	}
	defer func(dst *sftp.File) {
		err := dst.Close()
		if err != nil {
			logger.Errorf("关闭远程文件失败: userID=%d, remotePath=%s, err=%v", userID, remotePath, err)
		}
	}(dst)

	// 使用较大的缓冲区进行复制，提高大文件传输效率
	buf := make([]byte, 1024*1024) // 1MB buffer
	if _, err := io.CopyBuffer(dst, file, buf); err != nil {
		logger.Errorf("写入文件失败: userID=%d, remotePath=%s, err=%v", userID, remotePath, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("写入文件失败: %s", remotePath), err)
	}

	// 成功上传后清除缓存
	s.invalidateFileListCache(userID, isRootMode, resolvedPath)

	return &dto.FileUploadData{
		Filename: header.Filename,
	}, nil
}

// DownloadFile 下载文件
func (s *fileService) DownloadFile(userID int, p string, isRootMode bool) (io.ReadCloser, string, int64, error) {
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", userID, isRootMode, err)
		return nil, "", 0, err
	}

	resolvedPath, err := s.resolvePath(userID, p, isRootMode)
	if err != nil {
		logger.Errorf("解析下载路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", userID, p, isRootMode, err)
		return nil, "", 0, err
	}

	f, err := client.Open(resolvedPath)
	if err != nil {
		logger.Errorf("打开文件失败: userID=%d, resolvedPath=%s, err=%v", userID, resolvedPath, err)
		return nil, "", 0, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("打开文件失败: %s", resolvedPath), err)
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		logger.Errorf("获取文件状态失败: userID=%d, resolvedPath=%s, err=%v", userID, resolvedPath, err)
		return nil, "", 0, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("获取文件状态失败: %s", resolvedPath), err)
	}

	return f, path.Base(resolvedPath), stat.Size(), nil
}

// DeleteFile 删除文件或目录
func (s *fileService) DeleteFile(userID int, p string, isRootMode bool) error {
	resolvedPath, err := s.resolvePath(userID, p, isRootMode)
	if err != nil {
		logger.Errorf("解析删除路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", userID, p, isRootMode, err)
		return err
	}

	// 1. 检查文件是否存在
	sftpClient, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		return err
	}
	_, err = sftpClient.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New(errors.CodeInvalidParam, "文件或目录不存在")
		}
		return errors.NewWithErr(errors.CodeInternalError, "获取文件状态失败", err)
	}

	// 2. 使用 rm -rf 强制删除，支持文件和目录
	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"
	cmd := fmt.Sprintf("rm -rf %s", safePath)

	_, err = s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		logger.Errorf("执行删除命令失败: userID=%d, cmd=%s, err=%v", userID, cmd, err)
		return errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, "执行删除命令失败", err)
	}

	// 3. 再次检查文件是否存在，确认删除成功
	if _, err := sftpClient.Stat(resolvedPath); err == nil {
		return errors.New(errors.CodeInternalError, "删除失败，可能权限不足")
	} else if !os.IsNotExist(err) {
		return errors.NewWithErr(errors.CodeInternalError, "验证删除结果失败", err)
	}

	// 成功删除后清除缓存
	// 注意：删除的是 path.Dir(resolvedPath) 下的条目
	s.invalidateFileListCache(userID, isRootMode, path.Dir(resolvedPath))

	return nil
}

// UnzipFile 解压文件
func (s *fileService) UnzipFile(userID int, req *request.UnzipRequest, isRootMode bool) error {
	resolvedPath, err := s.resolvePath(userID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("解析解压源路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", userID, req.Path, isRootMode, err)
		return err
	}
	resolvedTarget, err := s.resolvePath(userID, req.TargetPath, isRootMode)
	if err != nil {
		logger.Errorf("解析解压目标路径失败: userID=%d, targetPath=%s, isRootMode=%t, err=%v", userID, req.TargetPath, isRootMode, err)
		return err
	}

	// 1. 检查目标文件/目录是否已存在，防止覆盖
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		return err
	}

	// 预测解压后的目录名（通常是压缩包文件名去掉扩展名）
	baseName := path.Base(resolvedPath)
	ext := path.Ext(baseName)
	expectedDirName := strings.TrimSuffix(baseName, ext)

	// 强制解压到以文件名命名的目录中
	finalTargetDir := path.Join(resolvedTarget, expectedDirName)

	// 检查 potentialPath 是否存在
	if _, err := client.Stat(finalTargetDir); err == nil {
		// 文件或目录已存在
		return errors.New(errors.CodeFileExists, fmt.Sprintf("目标路径已存在文件或目录: %s", expectedDirName))
	} else if !os.IsNotExist(err) {
		// 其他错误
		return errors.NewWithErr(errors.CodeInternalError, "检查文件状态失败", err)
	}

	// 2. 执行解压
	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"
	// 使用 finalTargetDir 作为解压目标
	safeTarget := "'" + strings.ReplaceAll(finalTargetDir, "'", "'\\''") + "'"

	// 确保目标目录存在（unzip -d 会自动创建目录，但我们显式创建更安全，或者依赖 unzip）
	// 注意：unzip -d target 会把内容解压到 target 下。
	cmd := fmt.Sprintf("unzip -n %s -d %s", safePath, safeTarget)
	_, err = s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		logger.Errorf("执行解压命令失败: userID=%d, cmd=%s, err=%v", userID, cmd, err)
		return errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, "执行解压命令失败", err)
	}

	// 成功解压后清除缓存
	// 解压到了 resolvedTarget 目录下（新增了一个子目录）
	s.invalidateFileListCache(userID, isRootMode, resolvedTarget)

	return nil
}

// GetDiskUsage 计算目录大小
func (s *fileService) GetDiskUsage(userID int, p string, isRootMode bool) (*dto.DiskUsageData, error) {
	resolvedPath, err := s.resolvePath(userID, p, isRootMode)
	if err != nil {
		logger.Errorf("解析磁盘使用路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", userID, p, isRootMode, err)
		return nil, err
	}

	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"

	var (
		size       int64
		totalBytes int64
		//wg         sync.WaitGroup
		errDu error
	)

	//wg.Add(2)

	// 1. 串行执行 du (不再并行，避免 SSH 通道竞争)
	//go func() {
	//	defer wg.Done()
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", safePath)
	output, err := s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		// 尝试回退方案
		if strings.Contains(err.Error(), "Permission denied") {
			errDu = errors.NewWithErr(errors.CodeForbidden, "权限不足，无法访问该目录", err)
			//return
		} else {
			// 尝试使用 du -sk 作为回退方案
			cmdFallback := fmt.Sprintf("du -sk %s | awk '{print $1}'", safePath)
			outputFallback, errFallback := s.executeSSHCommand(userID, cmdFallback, isRootMode)
			if errFallback == nil && strings.TrimSpace(outputFallback) != "" {
				output = outputFallback
				// 标记需要转换为字节（du -sk 输出的是 KB）
				kbStr := strings.TrimSpace(output)
				fields := strings.Fields(kbStr)
				if len(fields) > 0 {
					kbStr = fields[0]
				}
				if kb, parseErr := strconv.ParseInt(kbStr, 10, 64); parseErr == nil {
					size = kb * 1024
					//return
				}
			} else {
				errDu = errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, fmt.Sprintf("获取目录大小失败: %v", err), err)
				//return
			}
		}
	} else {
		sizeStr := strings.TrimSpace(output)
		if sizeStr == "" {
			size = 0
			//return
		} else {
			fields := strings.Fields(sizeStr)
			if len(fields) > 0 {
				sizeStr = fields[0]
			}
			if s, parseErr := strconv.ParseInt(sizeStr, 10, 64); parseErr == nil {
				size = s
			} else {
				logger.Errorf("解析目录大小失败: userID=%d, sizeStr=%s, err=%v", userID, sizeStr, parseErr)
				errDu = errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("解析目录大小数据失败: %s", sizeStr), parseErr)
			}
		}
	}
	//}()

	// 如果 du 失败，直接返回错误，不再执行 df
	if errDu != nil {
		return nil, errDu
	}

	// 2. 串行执行 df
	//go func() {
	//	defer wg.Done()
	// 直接执行 df -kP，不使用管道，在 Go 中解析
	cmdTotal := fmt.Sprintf("df -kP %s", safePath)
	outTotal, err := s.executeSSHCommand(userID, cmdTotal, isRootMode)
	if err == nil {
		lines := strings.Split(strings.TrimSpace(outTotal), "\n")
		if len(lines) > 0 {
			// 取最后一行
			lastLine := lines[len(lines)-1]
			fields := strings.Fields(lastLine)

			// df -kP 输出通常是：Filesystem 1024-blocks Used Available Capacity Mounted on
			// 至少有 6 列，或者是 5 列（如果 Filesystem 很长换行了）
			// 我们尝试解析第2列（Total），或者倒数第5列（Total）

			var parsed bool
			// 尝试 1：取第2列
			if len(fields) >= 2 {
				if t, parseErr := strconv.ParseInt(fields[1], 10, 64); parseErr == nil {
					totalBytes = t * 1024
					parsed = true
				}
			}

			// 尝试 2：如果第2列解析失败，或者列数很多，尝试取倒数第5列
			// 因为有时候 Filesystem 包含空格（虽然不太常见）或者输出被换行
			// 标准 POSIX df 输出最后 5 列是固定的：Blocks Used Available Capacity Mounted
			if !parsed && len(fields) >= 5 {
				if t, parseErr := strconv.ParseInt(fields[len(fields)-5], 10, 64); parseErr == nil {
					totalBytes = t * 1024
					parsed = true
				}
			}

			if !parsed {
				logger.Errorf("解析磁盘总大小数值失败: userID=%d, line=%s", userID, lastLine)
			}
		} else {
			logger.Errorf("df输出行数不足: userID=%d, output=%s", userID, outTotal)
		}
	} else {
		logger.Errorf("执行df命令获取总量失败: userID=%d, cmd=%s, err=%v", userID, cmdTotal, err)
	}
	//}()

	//wg.Wait()

	if errDu != nil {
		return nil, errDu
	}

	var usage float64
	if totalBytes > 0 {
		usage = float64(size) / float64(totalBytes) * 100
	}

	return &dto.DiskUsageData{
		DirectorySize: size,
		DiskUsage:     usage,
	}, nil
}

// CalculateSize 计算目录或文件大小
func (s *fileService) CalculateSize(userID int, p string, isRootMode bool) (int64, string, float64, error) {
	resolvedPath, err := s.resolvePath(userID, p, isRootMode)
	if err != nil {
		logger.Errorf("解析计算大小路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", userID, p, isRootMode, err)
		return 0, "", 0, err
	}

	quotedPath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"

	// 1. 使用 du -sb 计算字节大小
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", quotedPath)
	out, err := s.executeSSHCommand(userID, cmd, isRootMode)

	var bytess int64

	if err != nil {
		logger.Errorf("执行du -sb命令失败，尝试du -sk: userID=%d, cmd=%s, err=%v", userID, cmd, err)

		// 检查是否是权限问题
		if strings.Contains(err.Error(), "Permission denied") {
			return 0, "", 0, errors.NewWithErr(errors.CodeForbidden, "权限不足，无法访问该文件或目录", err)
		}

		// 如果 du -sb 失败，尝试 du -sk (千字节)
		cmd = fmt.Sprintf("du -sk %s | awk '{print $1}'", quotedPath)
		out, err = s.executeSSHCommand(userID, cmd, isRootMode)
		if err != nil {
			logger.Errorf("执行du -sk命令失败: userID=%d, cmd=%s, err=%v", userID, cmd, err)
			return 0, "", 0, errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, fmt.Sprintf("计算大小失败: %v", err), err)
		}
		// 转换为字节
		kbStr := strings.TrimSpace(out)
		kb, parseErr := strconv.ParseInt(kbStr, 10, 64)
		if parseErr != nil {
			logger.Errorf("解析千字节大小失败: userID=%d, kbStr=%s, err=%v", userID, kbStr, parseErr)
			return 0, "", 0, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("解析大小数据失败: %s", kbStr), parseErr)
		}
		bytess = kb * 1024
	} else {
		sizeStr := strings.TrimSpace(out)
		// 修复：如果 du -sb 输出包含文件名，只取第一列
		fields := strings.Fields(sizeStr)
		if len(fields) > 0 {
			sizeStr = fields[0]
		}
		bytess, err = strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			logger.Errorf("解析字节大小失败: userID=%d, sizeStr=%s, err=%v", userID, sizeStr, err)
			return 0, "", 0, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("解析大小数据失败: %s", sizeStr), err)
		}
	}

	// 2. 使用 df 获取磁盘总大小和可用空间，计算占比
	cmdTotal := fmt.Sprintf("df -kP %s", quotedPath)
	outTotal, err := s.executeSSHCommand(userID, cmdTotal, isRootMode)

	var usagePercent float64
	if err == nil {
		lines := strings.Split(strings.TrimSpace(outTotal), "\n")
		if len(lines) > 0 {
			// 取最后一行
			lastLine := lines[len(lines)-1]
			fields := strings.Fields(lastLine)

			var totalBytes int64
			var parsed bool

			// df -kP 输出通常是：Filesystem 1024-blocks Used Available Capacity Mounted on
			// 至少有 6 列，或者是 5 列（如果 Filesystem 很长换行了）
			// 我们尝试解析第2列（Total），或者倒数第5列（Total）

			// 尝试 1：取第2列
			if len(fields) >= 2 {
				if t, parseErr := strconv.ParseInt(fields[1], 10, 64); parseErr == nil {
					totalBytes = t * 1024
					parsed = true
				}
			}

			// 尝试 2：如果第2列解析失败，或者列数很多，尝试取倒数第5列
			if !parsed && len(fields) >= 5 {
				if t, parseErr := strconv.ParseInt(fields[len(fields)-5], 10, 64); parseErr == nil {
					totalBytes = t * 1024
					parsed = true
				}
			}

			if parsed {
				if totalBytes > 0 {
					usagePercent = float64(bytess) / float64(totalBytes) * 100
				}
			} else {
				logger.Errorf("解析磁盘总大小数值失败: userID=%d, line=%s", userID, lastLine)
			}
		} else {
			logger.Errorf("df输出行数不足: userID=%d, output=%s", userID, outTotal)
		}
	} else {
		logger.Errorf("执行df命令获取总量失败: userID=%d, cmd=%s, err=%v", userID, cmdTotal, err)
	}

	return bytess, formatFileSize(bytess), usagePercent, nil
}

// GetHomeDirectoriesList 获取 /home 目录下的所有用户目录列表
func (s *fileService) GetHomeDirectoriesList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error) {
	opUserID := userID

	// 启动对应的ftpclient，以root模式获取，因为要访问 /home
	client, err := s.getSftpClient(opUserID, true) // isRootMode = true
	if err != nil {
		return nil, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	// 解析路径并限制在 base_path 目录下
	basePath := config.Get().Server.BasePath
	if basePath == "" {
		basePath = "/home"
	}
	var currentPath string
	if req.Path == "" || req.Path == "." || req.Path == "/" {
		currentPath = basePath
	} else {
		// 拼接并清理路径
		joinedPath := path.Join(basePath, req.Path)
		cleanPath := path.Clean(joinedPath)

		// 确保路径没有越权
		if !strings.HasPrefix(cleanPath, basePath) {
			return nil, errors.New(errors.CodeForbidden, fmt.Sprintf("越权访问被拒绝: %s", req.Path))
		}
		currentPath = cleanPath
	}

	realPath := currentPath

	// 读取目标目录下的所有文件和目录
	entries, err := client.ReadDir(realPath)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("读取目录失败: %s", realPath), err)
	}

	var dirs []*dto.DirectoryItem

	// 构建 UID -> Username 映射
	uidToName := make(map[uint32]string)
	passwdOut, err := s.executeSSHCommand(opUserID, "getent passwd || cat /etc/passwd", isRootMode) // isRootMode = true
	if err == nil {
		lines := strings.Split(passwdOut, "\n")
		for _, line := range lines {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				username := parts[0]
				uidStr := parts[2]
				if uid, err := strconv.ParseUint(uidStr, 10, 32); err == nil {
					uidToName[uint32(uid)] = username
				}
			}
		}
	}

	for _, entry := range entries {
		// 只列出目录
		if entry.IsDir() {
			if req.Keyword != "" && !strings.Contains(entry.Name(), req.Keyword) {
				continue
			}

			var owner string
			if sys := entry.Sys(); sys != nil {
				if stat, ok := sys.(*sftp.FileStat); ok {
					name, found := uidToName[stat.UID]
					if found {
						owner = name
					} else {
						owner = fmt.Sprintf("%d", stat.UID)
					}
				} else {
					owner = "unknown"
				}
			} else {
				owner = "unknown"
			}

			dirs = append(dirs, &dto.DirectoryItem{
				Name:      entry.Name(),
				UpdatedAt: entry.ModTime(),
				Path:      path.Join(realPath, entry.Name()),
				Owner:     owner,
			})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })

	// Pagination
	totalDirs := len(dirs)
	total := totalDirs
	pages := total / req.PageSize
	if total%req.PageSize != 0 {
		pages++
	}

	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if end > total {
		end = total
	}

	var slicedDirs []*dto.DirectoryItem
	if start < total {
		slicedDirs = dirs[start:end]
	}

	// Build Breadcrumb for /home
	breadcrumb := buildBreadcrumb(realPath)

	return &dto.FilesListData{
		Breadcrumb:    breadcrumb,
		Path:          realPath,
		FileList:      []*dto.FileItem{}, // No files in this view
		DirectoryList: slicedDirs,
		Total:         total,
		Pages:         pages,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}, nil
}

// formatFileSize 格式化文件大小
func formatFileSize(s int64) string {
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	if s == 0 {
		return "0 B"
	}
	humanfmt := float64(s)
	i := 0
	for humanfmt >= 1024 && i < len(sizes)-1 {
		humanfmt /= 1024
		i++
	}
	return fmt.Sprintf("%.2f %s", humanfmt, sizes[i])
}

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskStatusPending   TaskStatus = 0
	TaskStatusRunning   TaskStatus = 1
	TaskStatusCompleted TaskStatus = 2
	TaskStatusFailed    TaskStatus = 3
)

// FileTask 文件操作任务
type FileTask struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"` // "unzip", "download", etc.
	Status    TaskStatus `json:"status"`
	Progress  int        `json:"progress"` // 0-100
	Error     string     `json:"error,omitempty"`
	Result    any        `json:"result,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	UserID    int        `json:"user_id"`
}

// AsyncTaskService 异步任务服务接口
type AsyncTaskService interface {
	SubmitTask(taskID string, task func() error)
	GetTask(taskID string) (*FileTask, bool)
	CreateTask(userID int, taskType string) *FileTask
	UpdateTaskStatus(taskID string, status TaskStatus, progress int, err error)
	SetPool(pool *websocket.ConnectionPool)
}

type asyncTaskService struct {
	tasks  sync.Map // map[string]*FileTask
	wsPool *websocket.ConnectionPool
}

var (
	globalAsyncTaskService *asyncTaskService
	once                   sync.Once
)

// GetAsyncTaskService 获取单例
func GetAsyncTaskService() AsyncTaskService {
	once.Do(func() {
		globalAsyncTaskService = &asyncTaskService{}
		// 启动清理过期任务的协程
		go globalAsyncTaskService.cleanupLoop()
	})
	return globalAsyncTaskService
}

func (s *asyncTaskService) SetPool(pool *websocket.ConnectionPool) {
	s.wsPool = pool
}

func (s *asyncTaskService) CreateTask(userID int, taskType string) *FileTask {
	taskID := fmt.Sprintf("task_%d_%d", userID, time.Now().UnixNano())
	task := &FileTask{
		ID:        taskID,
		Type:      taskType,
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    userID,
	}
	s.tasks.Store(taskID, task)
	return task
}

func (s *asyncTaskService) SubmitTask(taskID string, taskFunc func() error) {
	go func() {
		// 更新为运行中
		s.UpdateTaskStatus(taskID, TaskStatusRunning, 0, nil)

		err := taskFunc()

		if err != nil {
			logger.Errorf("异步任务执行失败: taskID=%s, err=%v", taskID, err)
			s.UpdateTaskStatus(taskID, TaskStatusFailed, 0, err)
		} else {
			logger.Infof("异步任务执行成功: taskID=%s", taskID)
			s.UpdateTaskStatus(taskID, TaskStatusCompleted, 100, nil)
		}
	}()
}

func (s *asyncTaskService) GetTask(taskID string) (*FileTask, bool) {
	if val, ok := s.tasks.Load(taskID); ok {
		return val.(*FileTask), true
	}
	return nil, false
}

func (s *asyncTaskService) UpdateTaskStatus(taskID string, status TaskStatus, progress int, err error) {
	if val, ok := s.tasks.Load(taskID); ok {
		task := val.(*FileTask)
		task.Status = status
		task.Progress = progress
		if err != nil {
			task.Error = err.Error()
		}
		task.UpdatedAt = time.Now()

		// 发送 WebSocket 通知
		s.sendNotification(task)
	}
}

// sendNotification 发送 WebSocket 通知
func (s *asyncTaskService) sendNotification(task *FileTask) {
	if s.wsPool == nil {
		return
	}

	msg := map[string]interface{}{
		"type": "task_update",
		"data": task,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logger.Errorf("JSON序列化失败: %v", err)
		return
	}

	s.wsPool.SendToUser(task.UserID, data)
}

// cleanupLoop 定期清理过期任务（例如保留1小时）
func (s *asyncTaskService) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		now := time.Now()
		s.tasks.Range(func(key, value any) bool {
			task := value.(*FileTask)
			// 如果任务完成或失败超过1小时，则清理
			if (task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed) &&
				now.Sub(task.UpdatedAt) > 1*time.Hour {
				s.tasks.Delete(key)
			}
			return true
		})
	}
}
