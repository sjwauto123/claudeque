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
	userRepo          repository.UserRepository
	homeDirCache      sync.Map
	sessionCheckCache sync.Map
}

// NewFileService 创建文件服务
func NewFileService(sessionManager *ssh.SessionManager, authService AuthService, redisRepo repository.RedisRepository, userRepo repository.UserRepository) FileService {
	return &fileService{
		sessionManager: sessionManager,
		authService:    authService,
		redisRepo:      redisRepo,
		userRepo:       userRepo,
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
		// 设置缓存时间为 3s，平衡性能与安全性
		needCheck := true
		if lastCheck, ok := s.sessionCheckCache.Load(userID); ok {
			if time.Since(lastCheck.(time.Time)) < 3*time.Second {
				needCheck = false
				isValid = true
			}
		}

		if needCheck {
			// 尝试轻量级 ping
			if _, pingErr := session.Client.ExecuteCommand("echo 1"); pingErr == nil {
				// 确保 SFTP 客户端也存在且未关闭
				if sftpClient := session.Client.GetSFTPClient(); sftpClient != nil {
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
		return nil, errors.NewWithErr(errors.CodeInternalError, "恢复SSH会话失败", err)
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
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, "获取SFTP客户端失败", err)
	}
	return session.Client.GetSFTPClient(), nil
}

// getSSHClient 获取用户的SSH客户端
func (s *fileService) getSSHClient(userID int, isRoot bool) (*server.Client, error) {
	session, err := s.getOrReconnectSession(userID, isRoot)
	if err != nil {
		logger.Errorf("获取SSH客户端失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, "获取SSH客户端失败", err)
	}
	return session.Client, nil
}

// executeSSHCommand 执行SSH命令
func (s *fileService) executeSSHCommand(userID int, cmd string, isRoot bool) (string, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("获取SSH客户端失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return "", errors.NewWithErr(errors.CodeInternalError, "获取SSH客户端失败", err)
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
		return "", errors.NewWithErr(errors.CodeInternalError, "获取SSH会话失败", err)
	}
	//
	//管理员模式:只需要处理为空的情况，不需要验证路径直接返回
	if isRootMode {
		//防止所传path为空的情况
		if p == "" || p == "." {
			return "/", nil
		}
		// 统一清理路径，移除末尾斜杠等
		return path.Clean(p), nil
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
		return nil, errors.NewWithErr(errors.CodeInternalError, "解析路径失败", err)
	}

	var dirs []*dto.DirectoryItem = []*dto.DirectoryItem{}
	var files []*dto.FileItem = []*dto.FileItem{}
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
			return nil, errors.NewWithErr(errors.CodeInternalError, "获取SFTP客户端失败", err)
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

		// 2. 如果缓存为空或命中率低，则执行命令加载（不再限制目录大小，确保始终能解析为用户名）
		if len(uidToName) == 0 {
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
				if s.redisRepo != nil && len(cachePairs) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					cacheVal := strings.Join(cachePairs, ",")
					_ = s.redisRepo.Set(ctx, cacheKey, cacheVal, 1*time.Hour)
					cancel()
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
						owner = entry.Name()
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
					Username:  owner,
				})
			} else {
				files = append(files, &dto.FileItem{
					Filename:  entry.Name(),
					FileSize:  entry.Size(),
					UpdatedAt: entry.ModTime(),
					Path:      path.Join(realPath, entry.Name()),
					Username:  owner,
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
				_ = s.redisRepo.Set(ctx, listCacheKey, string(data), 5*time.Second)
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

	var resultDirs []*dto.DirectoryItem = []*dto.DirectoryItem{}
	var resultFiles []*dto.FileItem = []*dto.FileItem{}

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
	// 确保临时上传目录存在
	tempDir := "uploads/temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		logger.Errorf("创建临时上传目录失败: %v", err)
		return nil, errors.New(errors.CodeInternalError, "创建临时目录失败")
	}

	// 创建一个唯一的临时文件
	tempFile, err := os.CreateTemp(tempDir, fmt.Sprintf("upload-%d-%s-", userID, header.Filename))
	if err != nil {
		logger.Errorf("创建临时文件失败: %v", err)
		return nil, errors.New(errors.CodeInternalError, "创建临时文件失败")
	}
	defer func(tempFile *os.File) {
		err := tempFile.Close()
		if err != nil {
			logger.Errorf("临时文件关闭失败")
		}
	}(tempFile)

	// 将上传的文件内容保存到临时文件
	_, err = io.Copy(tempFile, file)
	if err != nil {
		logger.Errorf("保存临时文件失败: %v", err)
		_ = os.Remove(tempFile.Name()) // 清理失败的临时文件
		return nil, errors.New(errors.CodeInternalError, "保存上传文件失败")
	}

	// 获取临时文件的完整路径
	tempFilePath := tempFile.Name()

	// 启动后台 goroutine 执行真正的上传
	go s.asyncUploadToSFTP(userID, tempFilePath, header.Filename, targetPath, isRootMode, header.Size)

	// 立即返回成功响应
	return &dto.FileUploadData{
		Filename: header.Filename,
	}, nil
}

// asyncUploadToSFTP 是一个在后台运行的函数，负责将临时文件上传到 SFTP
func (s *fileService) asyncUploadToSFTP(userID int, tempFilePath, originalFilename, targetPath string, isRootMode bool, totalSize int64) {
	// 函数结束时删除临时文件
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			logger.Errorf("删除对应文件失败")
		}
	}(tempFilePath)

	// 准备进度上报
	progressKey := s.getUploadProgressKey(userID, originalFilename, targetPath, isRootMode)
	reportProgress := func(status string, uploaded int64, speed, remaining, errMsg string) {
		if status == "failed" && errMsg != "" {
			logger.Errorf("文件上传失败: userID=%d, filename=%s, targetPath=%s, error=%s", userID, originalFilename, targetPath, errMsg)
		}
		if s.redisRepo == nil {
			return
		}
		progress := 0.0
		if totalSize > 0 {
			progress = float64(uploaded) / float64(totalSize) * 100
		}
		data := &dto.UploadProgressData{
			Filename:  originalFilename,
			Status:    status,
			Progress:  progress,
			Uploaded:  uploaded,
			Total:     totalSize,
			Speed:     speed,
			Remaining: remaining,
			Error:     errMsg,
		}
		jsonData, _ := json.Marshal(data)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.redisRepo.Set(ctx, progressKey, string(jsonData), 1*time.Hour) // 进度信息保留1小时
	}

	// 1. 获取 SFTP 客户端
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		reportProgress("failed", 0, "", "", "获取SFTP客户端失败: "+err.Error())
		return
	}

	// 2. 解析最终目标路径
	resolvedPath, err := s.resolvePath(userID, targetPath, isRootMode)
	if err != nil {
		reportProgress("failed", 0, "", "", "解析目标路径失败: "+err.Error())
		return
	}

	// 3. 确保目标目录存在
	if err := client.MkdirAll(resolvedPath); err != nil {
		reportProgress("failed", 0, "", "", "创建SFTP目录失败: "+err.Error())
		return
	}

	// 4. 创建远程文件
	dstPath := path.Join(resolvedPath, originalFilename)
	dstFile, err := client.Create(dstPath)
	if err != nil {
		reportProgress("failed", 0, "", "", "创建远程文件失败: "+err.Error())
		return
	}
	defer func(dstFile *sftp.File) {
		err := dstFile.Close()
		if err != nil {
			logger.Errorf("远程文件关闭失败")
		}
	}(dstFile)

	// 5. 打开本地临时文件准备读取
	localFile, err := os.Open(tempFilePath)
	if err != nil {
		reportProgress("failed", 0, "", "", "打开临时文件失败: "+err.Error())
		return
	}
	defer func(localFile *os.File) {
		err := localFile.Close()
		if err != nil {
			logger.Errorf("本地文件删除失败")
		}
	}(localFile)

	// 6. 创建带进度跟踪的 Reader
	progressReader := &ProgressReader{
		Reader: localFile,
		Total:  totalSize,
		ReportFunc: func(uploaded int64, speed, remaining string) {
			reportProgress("uploading", uploaded, speed, remaining, "")
		},
	}

	// 7. 开始流式上传
	buf := make([]byte, 64*1024) // 64KB buffer
	_, err = io.CopyBuffer(dstFile, progressReader, buf)
	if err != nil {
		_ = client.Remove(dstPath) // 清理不完整的文件
		reportProgress("failed", progressReader.Uploaded, "", "", "上传失败: "+err.Error())
		return
	}

	// 8. 上传完成
	reportProgress("completed", totalSize, "", "", "")

	// 9. 清理文件列表缓存
	s.invalidateFileListCache(userID, isRootMode, resolvedPath)
}

// getUploadProgressKey 生成用于 Redis 的进度键
func (s *fileService) getUploadProgressKey(userID int, filename, targetPath string, isRootMode bool) string {
	// 为了保证键的唯一性，我们需要一个稳定的、能代表这次上传任务的字符串。
	// 这里的实现依赖于 `resolvePath` 来获取确定性的目标路径。
	// 注意：这是一个简化实现，如果 resolvePath 逻辑很重，可能会影响性能。
	// 在高并发场景下，可能需要更轻量的键生成策略。
	resolvedPath, err := s.resolvePath(userID, targetPath, isRootMode)
	if err != nil {
		// 如果解析失败，使用原始路径，但这可能导致键不一致
		resolvedPath = targetPath
	}
	return fmt.Sprintf("upload:progress:%d:%s:%s", userID, filename, resolvedPath)
}

// ProgressReader 是一个包装器，用于在读取时报告进度
type ProgressReader struct {
	Reader     io.Reader
	Total      int64
	Uploaded   int64
	ReportFunc func(uploaded int64, speed, remaining string)
	lastReport time.Time
	startTime  time.Time
}

func (r *ProgressReader) Read(p []byte) (n int, err error) {
	if r.startTime.IsZero() {
		r.startTime = time.Now()
		r.lastReport = time.Now()
	}

	n, err = r.Reader.Read(p)
	r.Uploaded += int64(n)

	// 每秒上报一次进度
	if time.Since(r.lastReport) > time.Second || (err == io.EOF && n > 0) {
		duration := time.Since(r.startTime).Seconds()
		var speedStr, remainingStr string
		if duration > 0 {
			bytesPerSec := float64(r.Uploaded) / duration
			speedStr = formatUploadSpeed(bytesPerSec)
			if r.Total > r.Uploaded {
				remBytes := float64(r.Total - r.Uploaded)
				remSeconds := remBytes / bytesPerSec
				remainingStr = formatUploadDuration(time.Duration(remSeconds) * time.Second)
			}
		}
		if r.ReportFunc != nil {
			r.ReportFunc(r.Uploaded, speedStr, remainingStr)
		}
		r.lastReport = time.Now()
	}

	return n, err
}
func (s *fileService) DownloadFile(userID int, p string, isRootMode bool) (io.ReadCloser, string, int64, time.Time, error) {
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", userID, isRootMode, err)
		return nil, "", 0, time.Time{}, err
	}

	resolvedPath, err := s.resolvePath(userID, p, isRootMode)
	if err != nil {
		logger.Errorf("解析下载路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", userID, p, isRootMode, err)
		return nil, "", 0, time.Time{}, err
	}

	f, err := client.Open(resolvedPath)
	if err != nil {
		logger.Errorf("打开文件失败: userID=%d, resolvedPath=%s, err=%v", userID, resolvedPath, err)
		return nil, "", 0, time.Time{}, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("打开文件失败: %s", resolvedPath), err)
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		logger.Errorf("获取文件状态失败: userID=%d, resolvedPath=%s, err=%v", userID, resolvedPath, err)
		return nil, "", 0, time.Time{}, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("获取文件状态失败: %s", resolvedPath), err)
	}

	return f, path.Base(resolvedPath), stat.Size(), stat.ModTime(), nil
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
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", userID, isRootMode, err)
		return err
	}
	_, err = sftpClient.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("文件或目录不存在: userID=%d, path=%s", userID, resolvedPath)
			return errors.New(errors.CodeInvalidParam, "文件或目录不存在")
		}
		logger.Errorf("获取文件状态失败: userID=%d, path=%s, err=%v", userID, resolvedPath, err)
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
		logger.Errorf("删除验证失败（文件仍存在）: userID=%d, path=%s", userID, resolvedPath)
		return errors.New(errors.CodeInternalError, "删除失败，可能权限不足")
	} else if !os.IsNotExist(err) {
		logger.Errorf("验证删除结果失败: userID=%d, path=%s, err=%v", userID, resolvedPath, err)
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
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", userID, isRootMode, err)
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
		logger.Warnf("解压目标路径已存在: userID=%d, target=%s", userID, finalTargetDir)
		return errors.New(errors.CodeFileExists, fmt.Sprintf("目标路径已存在文件或目录: %s", expectedDirName))
	} else if !os.IsNotExist(err) {
		// 其他错误
		logger.Errorf("检查解压目标状态失败: userID=%d, target=%s, err=%v", userID, finalTargetDir, err)
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

// GetDiskUsage 计算目录大小及磁盘占用
func (s *fileService) GetDiskUsage(userID int, p string, isRootMode bool) (*dto.DiskUsageData, error) {
	// 1. 尝试从 Redis 获取缓存
	ctx := context.Background()
	cacheData, err := s.GetDiskUsageCache(ctx, userID, p)
	if err == nil && cacheData != nil {
		// 缓存存在，返回缓存数据
		return cacheData, nil
	}

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
	cmd := fmt.Sprintf("du -sb %s 2>/dev/null | awk '{print $1}'", safePath)
	output, err := s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		// 尝试回退方案
		outStr := strings.TrimSpace(output)
		if strings.Contains(err.Error(), "Permission denied") || strings.Contains(outStr, "Permission denied") {
			errDu = errors.NewWithErr(errors.CodeForbidden, "权限不足，无法访问该目录", err)
		} else {
			// 尝试使用 du -sk 作为回退方案
			cmdFallback := fmt.Sprintf("du -sk %s 2>/dev/null | awk '{print $1}'", safePath)
			outputFallback, errFallback := s.executeSSHCommand(userID, cmdFallback, isRootMode)
			if errFallback == nil && strings.TrimSpace(outputFallback) != "" {
				output = outputFallback
				// 标记需要转换为字节（du -sk 输出的是 KB）
				kbStr := strings.TrimSpace(output)
				// 取最后一行，避免其他输出干扰
				lines := strings.Split(kbStr, "\n")
				lastLine := ""
				for i := len(lines) - 1; i >= 0; i-- {
					l := strings.TrimSpace(lines[i])
					if l != "" {
						lastLine = l
						break
					}
				}
				fields := strings.Fields(lastLine)
				if len(fields) > 0 {
					kbStr = fields[0]
				}
				if kb, parseErr := strconv.ParseInt(kbStr, 10, 64); parseErr == nil {
					size = kb * 1024
				}
			} else {
				errDu = errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, fmt.Sprintf("获取目录大小失败: %v, output: %s", err, output), err)
			}
		}
	} else {
		sizeStr := strings.TrimSpace(output)
		if sizeStr == "" {
			// 如果 output 为空但 err 为 nil，可能是管道问题，尝试直接 du -sb
			cmdDirect := fmt.Sprintf("du -sb %s 2>/dev/null", safePath)
			outDirect, errDirect := s.executeSSHCommand(userID, cmdDirect, isRootMode)
			if errDirect == nil && strings.TrimSpace(outDirect) != "" {
				sizeStr = strings.TrimSpace(outDirect)
			} else {
				size = 0
				logger.Warnf("du 命令返回空输出: userID=%d, path=%s", userID, safePath)
			}
		}

		if sizeStr != "" {
			// 取最后一行，避免其他输出干扰
			lines := strings.Split(sizeStr, "\n")
			lastLine := ""
			for i := len(lines) - 1; i >= 0; i-- {
				l := strings.TrimSpace(lines[i])
				if l != "" {
					lastLine = l
					break
				}
			}
			fields := strings.Fields(lastLine)
			if len(fields) > 0 {
				sizeStr = fields[0]
			}
			if sVal, parseErr := strconv.ParseInt(sizeStr, 10, 64); parseErr == nil {
				size = sVal
			} else {
				logger.Errorf("解析目录大小失败: userID=%d, sizeStr=%s, output=%q, err=%v", userID, sizeStr, output, parseErr)
				errDu = errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("解析目录大小数据失败: %s", sizeStr), parseErr)
			}
		}
	}

	// 如果 du 失败，直接返回错误，不再执行 df
	if errDu != nil {
		logger.Errorf("获取目录大小最终失败: userID=%d, path=%s, err=%v", userID, resolvedPath, errDu)
		return nil, errDu
	}

	// 2. 优先使用 SFTP StatVFS 获取磁盘总量 (更可靠，不依赖 shell 解析)
	sftpClient, errSftp := s.getSftpClient(userID, isRootMode)
	if errSftp == nil {
		if statvfs, errStatVfs := sftpClient.StatVFS(resolvedPath); errStatVfs == nil {
			totalBytes = int64(statvfs.Blocks) * int64(statvfs.Bsize)
			logger.Debugf("通过 SFTP StatVFS 获取磁盘总量成功: userID=%d, path=%s, totalBytes=%d", userID, resolvedPath, totalBytes)
		} else {
			logger.Warnf("SFTP StatVFS 获取失败，回退到 df: userID=%d, path=%s, err=%v", userID, resolvedPath, errStatVfs)
		}
	}

	// 3. 如果 StatVFS 失败，回退到原来的 df 方案
	if totalBytes == 0 {
		cmdTotal := fmt.Sprintf("df -kP %s", safePath)
		outTotal, err := s.executeSSHCommand(userID, cmdTotal, isRootMode)
		if err == nil {
			// 优化解析逻辑
			lines := strings.Split(strings.TrimSpace(outTotal), "\n")
			var targetLine string
			// 倒序查找第一个非表头、非空的行
			for i := len(lines) - 1; i >= 0; i-- {
				line := strings.TrimSpace(lines[i])
				if line == "" {
					continue
				}
				// 跳过表头
				if strings.Contains(line, "Filesystem") || strings.Contains(line, "1024-blocks") {
					continue
				}
				targetLine = line
				break
			}

			if targetLine != "" {
				fields := strings.Fields(targetLine)
				// 策略：寻找带有 % 的 Capacity 列，Total 通常在它前面第3列
				// Filesystem Total Used Avail Cap% Mounted
				capIndex := -1
				for i := len(fields) - 1; i >= 0; i-- {
					if strings.HasSuffix(fields[i], "%") {
						capIndex = i
						break
					}
				}

				var parsed bool
				// 尝试 1：基于 Capacity 位置推断
				if capIndex >= 3 {
					if t, err := strconv.ParseInt(fields[capIndex-3], 10, 64); err == nil {
						totalBytes = t * 1024
						parsed = true
					}
				}

				// 尝试 2：如果找不到 %，或者解析失败，尝试倒数第5列 (标准 POSIX)
				if !parsed && len(fields) >= 5 {
					// 倒数第5列通常是 Total
					if t, err := strconv.ParseInt(fields[len(fields)-5], 10, 64); err == nil {
						totalBytes = t * 1024
						parsed = true
					}
				}

				if !parsed {
					logger.Errorf("解析磁盘总大小数值失败: userID=%d, line=%s, outTotal=%q", userID, targetLine, outTotal)
				}
			} else {
				logger.Errorf("df输出未找到有效数据行: userID=%d, output=%q", userID, outTotal)
			}
		} else {
			logger.Errorf("执行df命令获取总量失败: userID=%d, cmd=%s, err=%v", userID, cmdTotal, err)
		}
	}

	// 4. 回退方案：如果 df 也未能解析到 totalBytes，尝试使用 stat -f 计算总字节数
	if totalBytes == 0 {
		// 增加通用 stat -f 尝试，不依赖特定 -c 格式，如果上面失败
		cmdStat := fmt.Sprintf(`stat -f -c "%%b %%S" %s || stat -f "%%b %%S" %s`, safePath, safePath)
		outStat, errStat := s.executeSSHCommand(userID, cmdStat, isRootMode)
		if errStat == nil && strings.TrimSpace(outStat) != "" {
			lines := strings.Split(strings.TrimSpace(outStat), "\n")
			// 取最后一行，防止多行输出
			parts := strings.Fields(lines[len(lines)-1])
			if len(parts) >= 2 {
				if b, errB := strconv.ParseInt(parts[0], 10, 64); errB == nil {
					if ssz, errS := strconv.ParseInt(parts[1], 10, 64); errS == nil {
						totalBytes = b * ssz
					}
				}
			}
		} else {
			logger.Errorf("stat -f 计算总字节数失败: userID=%d, cmd=%s, err=%v, out=%q", userID, cmdStat, errStat, outStat)
		}
	}

	var usage float64
	if totalBytes > 0 {
		usage = float64(size) / float64(totalBytes) * 100
	}

	result := &dto.DiskUsageData{
		DirectorySize: size,
		DiskUsage:     usage,
	}

	// 3. 计算完成后，更新 Redis 缓存
	_ = s.SetDiskUsageCache(ctx, userID, p, result)

	return result, nil
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
		outStr := strings.TrimSpace(out)
		logger.Errorf("执行du -sb命令失败，尝试du -sk: userID=%d, cmd=%s, err=%v, output=%q", userID, cmd, err, outStr)

		// 检查是否是权限问题
		if strings.Contains(err.Error(), "Permission denied") || strings.Contains(outStr, "Permission denied") {
			return 0, "", 0, errors.NewWithErr(errors.CodeForbidden, "权限不足，无法访问该文件或目录", err)
		}

		// 如果 du -sb 失败，尝试 du -sk (千字节)
		cmd = fmt.Sprintf("du -sk %s | awk '{print $1}'", quotedPath)
		out, err = s.executeSSHCommand(userID, cmd, isRootMode)
		if err != nil {
			logger.Errorf("执行du -sk命令失败: userID=%d, cmd=%s, err=%v, output=%q", userID, cmd, err, out)
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
		if sizeStr == "" {
			// 尝试直接 du -sb
			cmdDirect := fmt.Sprintf("du -sb %s", quotedPath)
			outDirect, errDirect := s.executeSSHCommand(userID, cmdDirect, isRootMode)
			if errDirect == nil && strings.TrimSpace(outDirect) != "" {
				sizeStr = strings.TrimSpace(outDirect)
			}
		}

		if sizeStr != "" {
			// 修复：如果 du -sb 输出包含文件名，只取第一列
			fields := strings.Fields(sizeStr)
			if len(fields) > 0 {
				sizeStr = fields[0]
			}
			var parseErr error
			bytess, parseErr = strconv.ParseInt(sizeStr, 10, 64)
			if parseErr != nil {
				logger.Errorf("解析字节大小失败: userID=%d, sizeStr=%s, output=%q, err=%v", userID, sizeStr, out, parseErr)
				return 0, "", 0, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("解析大小数据失败: %s", sizeStr), parseErr)
			}
		} else {
			bytess = 0
			logger.Warnf("du 命令在 CalculateSize 返回空输出: userID=%d, path=%s", userID, quotedPath)
		}
	}

	// 2. 优先使用 SFTP StatVFS 获取磁盘总量
	sftpClient, errSftp := s.getSftpClient(userID, isRootMode)
	var totalBytes int64
	var usagePercent float64
	if errSftp == nil {
		if statvfs, errStatVfs := sftpClient.StatVFS(resolvedPath); errStatVfs == nil {
			totalBytes = int64(statvfs.Blocks) * int64(statvfs.Bsize)
			if totalBytes > 0 {
				usagePercent = float64(bytess) / float64(totalBytes) * 100
			}
		}
	}

	// 3. 如果 StatVFS 失败，使用 df 获取磁盘总大小和可用空间，计算占比
	if totalBytes == 0 {
		cmdTotal := fmt.Sprintf("df -kP %s", quotedPath)
		outTotal, err := s.executeSSHCommand(userID, cmdTotal, isRootMode)

		if err == nil {
			// 优化解析逻辑
			lines := strings.Split(strings.TrimSpace(outTotal), "\n")
			var targetLine string
			// 倒序查找第一个非表头、非空的行
			for i := len(lines) - 1; i >= 0; i-- {
				line := strings.TrimSpace(lines[i])
				if line == "" {
					continue
				}
				// 跳过表头
				if strings.Contains(line, "Filesystem") || strings.Contains(line, "1024-blocks") {
					continue
				}
				targetLine = line
				break
			}

			if targetLine != "" {
				fields := strings.Fields(targetLine)
				// 策略：寻找带有 % 的 Capacity 列，Total 通常在它前面第3列
				// Filesystem Total Used Avail Cap% Mounted
				capIndex := -1
				for i := len(fields) - 1; i >= 0; i-- {
					if strings.HasSuffix(fields[i], "%") {
						capIndex = i
						break
					}
				}

				var parsed bool

				// 尝试 1：基于 Capacity 位置推断
				if capIndex >= 3 {
					if t, err := strconv.ParseInt(fields[capIndex-3], 10, 64); err == nil {
						totalBytes = t * 1024
						parsed = true
					}
				}

				// 尝试 2：如果找不到 %，或者解析失败，尝试倒数第5列 (标准 POSIX)
				if !parsed && len(fields) >= 5 {
					if t, err := strconv.ParseInt(fields[len(fields)-5], 10, 64); err == nil {
						totalBytes = t * 1024
						parsed = true
					}
				}

				if parsed {
					if totalBytes > 0 {
						usagePercent = float64(bytess) / float64(totalBytes) * 100
					}
				} else {
					logger.Errorf("解析磁盘总大小数值失败: userID=%d, line=%s, outTotal=%q", userID, targetLine, outTotal)
				}
			} else {
				logger.Errorf("df输出未找到有效数据行: userID=%d, output=%q", userID, outTotal)
			}
		} else {
			logger.Errorf("执行df命令获取总量失败: userID=%d, cmd=%s, err=%v", userID, cmdTotal, err)
		}
	}

	// 4. 回退方案：df 失败或未解析到 totalBytes 时，使用 stat -f 计算总字节数
	if usagePercent == 0 && totalBytes == 0 {
		cmdStat := fmt.Sprintf(`stat -f -c "%%b %%S" %s || stat -f "%%b %%S" %s`, quotedPath, quotedPath)
		outStat, errStat := s.executeSSHCommand(userID, cmdStat, isRootMode)
		if errStat == nil && strings.TrimSpace(outStat) != "" {
			lines := strings.Split(strings.TrimSpace(outStat), "\n")
			// 取最后一行
			parts := strings.Fields(lines[len(lines)-1])
			if len(parts) >= 2 {
				if b, errB := strconv.ParseInt(parts[0], 10, 64); errB == nil {
					if ssz, errS := strconv.ParseInt(parts[1], 10, 64); errS == nil {
						total := b * ssz
						if total > 0 {
							usagePercent = float64(bytess) / float64(total) * 100
						}
					}
				}
			}
		} else {
			logger.Errorf("stat -f 计算总字节数失败: userID=%d, cmd=%s, err=%v, out=%q", userID, cmdStat, errStat, outStat)
		}
	}

	return bytess, formatFileSize(bytess), usagePercent, nil
}

// GetHomeDirectoriesList 获取 /home 目录下的所有用户目录列表
func (s *fileService) GetHomeDirectoriesList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error) {
	opUserID := userID

	// 启动对应的ftpclient，以root模式获取，因为要访问 /home
	client, err := s.getSftpClient(opUserID, true) // isRootMode = true
	if err != nil {
		logger.Errorf("获取SFTP客户端失败: userID=%d, err=%v", opUserID, err)
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
			logger.Errorf("越权访问被拒绝: userID=%d, path=%s, basePath=%s", opUserID, req.Path, basePath)
			return nil, errors.New(errors.CodeForbidden, fmt.Sprintf("越权访问被拒绝: %s", req.Path))
		}
		currentPath = cleanPath
	}

	realPath := currentPath

	// 读取目标目录下的所有文件 and 目录
	entries, err := client.ReadDir(realPath)
	if err != nil {
		logger.Errorf("读取目录失败: userID=%d, realPath=%s, err=%v", opUserID, realPath, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("读取目录失败: %s", realPath), err)
	}

	var dirs []*dto.DirectoryItem = []*dto.DirectoryItem{}

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
	} else {
		logger.Warnf("获取用户列表失败(非致命): userID=%d, err=%v", opUserID, err)
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
						owner = "暂无数据，请刷新"
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
				Username:  owner,
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

	var slicedDirs []*dto.DirectoryItem = []*dto.DirectoryItem{}
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

// GetUploadProgress 获取上传进度
func (s *fileService) GetUploadProgress(userID int, filename string, targetPath string) (*dto.UploadProgressData, error) {
	if s.redisRepo == nil {
		logger.Error("未配置进度存储 (Redis 未初始化)")
		return nil, errors.New(errors.CodeInternalError, "未配置进度存储")
	}

	filename = strings.TrimSpace(filename)
	targetPath = strings.TrimSpace(targetPath)

	// 1. 获取用户信息，判断是否是 root 模式（与上传逻辑保持一致）
	isRootMode, err := s.authService.HasSystemAccess(userID, AccessTypeFile)
	if err != nil {
		logger.Errorf("获取权限状态失败: userID=%d, err=%v", userID, err)
		return nil, err
	}

	// 2. 获取进度键
	progressKey := s.getUploadProgressKey(userID, filename, targetPath, isRootMode)

	// 3. 从 Redis 获取进度信息
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	val, err := s.redisRepo.Get(ctx, progressKey)
	if err != nil || val == "" {
		// 如果键不存在，可能任务已完成或从未开始
		return &dto.UploadProgressData{
			Filename: filename,
			Status:   "unknown",
		}, nil
	}

	var out dto.UploadProgressData
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		logger.Errorf("解析进度数据失败: userID=%d, progressKey=%s, err=%v", userID, progressKey, err)
		return nil, errors.NewWithErr(errors.CodeInternalError, "解析进度数据失败", err)
	}

	return &out, nil
}

func formatUploadSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.1f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/1024/1024)
	}
}

func formatUploadDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
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

// SetDiskUsageCache 设置磁盘使用缓存
func (s *fileService) SetDiskUsageCache(ctx context.Context, userID int, path string, data *dto.DiskUsageData) error {
	key := fmt.Sprintf("disk:usage:%d:%s", userID, path)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.redisRepo.Set(ctx, key, string(jsonData), 25*time.Hour)
}

// GetDiskUsageCache 获取磁盘使用缓存
func (s *fileService) GetDiskUsageCache(ctx context.Context, userID int, path string) (*dto.DiskUsageData, error) {
	key := fmt.Sprintf("disk:usage:%d:%s", userID, path)
	val, err := s.redisRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var data dto.DiskUsageData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// calculateDiskUsageWithRoot 使用 root 会话计算指定用户的磁盘使用情况
// 避免为每个用户建立 SSH 会话，直接用 root 权限访问任意用户目录
func (s *fileService) calculateDiskUsageWithRoot(username, homeDir string) (*dto.DiskUsageData, error) {
	logger.Infof("[DEBUG] 开始计算用户磁盘使用: username=%s, homeDir=%s", username, homeDir)

	if s.sessionManager == nil {
		logger.Error("[DEBUG] SSH会话管理器未初始化")
		return nil, errors.New(errors.CodeInternalError, "SSH会话管理器未初始化")
	}

	// 使用 root 用户名获取 root 会话（不需要用户登录凭证）
	rootUsername := s.sessionManager.GetRootUsername()
	logger.Infof("[DEBUG] root用户名: %s", rootUsername)

	// 尝试获取 root 会话，如果存在则检查是否可用
	var session *ssh.UserSession
	var err error
	session, err = s.sessionManager.GetSession(0, true)
	logger.Infof("[DEBUG] 获取root会话结果: session=%v, err=%v", session != nil, err)

	// 如果会话存在，执行一个简单命令检查会话是否正常
	if err == nil && session != nil && session.Client != nil {
		testCmd := "echo 'health_check'"
		output, testErr := session.Client.ExecuteCommand(testCmd)
		if testErr != nil || strings.TrimSpace(output) != "health_check" {
			logger.Warnf("[DEBUG] SSH会话健康检查失败，尝试重新创建: err=%v, output=%s", testErr, output)
			// 健康检查失败，销毁旧会话
			_ = s.sessionManager.DeleteSession(0, true)
			session = nil
		} else {
			logger.Info("[DEBUG] SSH会话健康检查成功")
		}
	}

	// 如果会话不存在或健康检查失败，创建新会话
	if session == nil {
		rootPassword := s.sessionManager.GetRootPassword()
		logger.Info("[DEBUG] 尝试创建新的root SSH会话")
		session, err = s.sessionManager.GetOrCreateSession(0, rootUsername, rootPassword, true)
		if err != nil {
			logger.Errorf("[DEBUG] 获取 root SSH 会话失败: err=%v", err)
			return nil, errors.NewWithErr(errors.CodeInternalError, "获取root会话失败", err)
		}
		logger.Info("[DEBUG] 成功创建root SSH会话")
	}

	if session == nil || session.Client == nil {
		logger.Error("[DEBUG] root SSH 会话不可用: session或client为nil")
		return nil, errors.New(errors.CodeInternalError, "root SSH 会话不可用")
	}

	// 直接使用 root 会话执行命令，无需用户凭证
	safePath := "'" + strings.ReplaceAll(homeDir, "'", "'\\''") + "'"
	logger.Infof("[DEBUG] 安全路径: %s", safePath)

	// 0. 先测试一个最简单的命令确认 SSH 是否正常
	logger.Infof("[DEBUG] ===== 测试 SSH 连接 =====")
	testSimpleCmd := "echo 'ssh_working'"
	logger.Infof("[DEBUG] 执行测试命令: %s", testSimpleCmd)
	testSimpleOutput, testSimpleErr := session.Client.ExecuteCommand(testSimpleCmd)
	logger.Infof("[DEBUG] SSH测试结果(普通): output=%q, err=%v", testSimpleOutput, testSimpleErr)

	logger.Infof("[DEBUG] ===== SSH 测试完成 =====")

	// 0. 先检查目录是否存在 (使用 ExecuteCommandWithStatus 代替 ExecuteCommand)
	checkCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'not_exists'", safePath)
	logger.Infof("[DEBUG] 检查目录是否存在: %s", checkCmd)
	checkOutput, checkExitCode, checkErr := session.Client.ExecuteCommandWithStatus(checkCmd)
	logger.Infof("[DEBUG] 检查目录命令执行完成: output=%q, exitCode=%d, err=%v", checkOutput, checkExitCode, checkErr)

	if checkErr != nil {
		logger.Errorf("[DEBUG] 检查目录命令执行失败: err=%v, output=%q", checkErr, checkOutput)
		return nil, errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, "检查目录失败", checkErr)
	}
	checkResult := strings.TrimSpace(checkOutput)
	logger.Infof("[DEBUG] 目录检查结果原始: %q, 去除空格后: %q", checkOutput, checkResult)

	if checkResult != "exists" {
		logger.Errorf("[DEBUG] !!! 目录不存在: username=%s, homeDir=%s, checkOutput=%q", username, homeDir, checkOutput)
		return nil, errors.NewWithErr(errors.CodeFileNotFound, fmt.Sprintf("用户目录不存在: %s", homeDir), nil)
	}
	logger.Infof("[DEBUG] 目录存在，继续计算磁盘使用")

	// 0.1 测试基本命令是否正常工作
	testCmd := "echo 'test_output'"
	logger.Infof("[DEBUG] 执行测试命令: %s", testCmd)
	testOutput, testErr := session.Client.ExecuteCommand(testCmd)
	if testErr != nil {
		logger.Errorf("[DEBUG] 测试命令执行失败: err=%v, output=%q", testErr, testOutput)
	} else {
		logger.Infof("[DEBUG] 测试命令输出: %q", testOutput)
	}

	// 1. 获取目录大小 - 不使用 2>/dev/null，以便看到错误信息
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", safePath)
	logger.Infof("[DEBUG] 执行目录大小命令: %s", cmd)

	// 使用 ExecuteCommandWithStatus 获取退出码
	output2, exitCode, execErr := session.Client.ExecuteCommandWithStatus(cmd)
	logger.Infof("[DEBUG] du命令结果: output=%q, exitCode=%d, err=%v", output2, exitCode, execErr)

	output := output2
	var duErr error
	if execErr != nil && exitCode == 0 {
		// 有错误但退出码为0，忽略
	} else if execErr != nil {
		duErr = execErr
	}

	if duErr != nil {
		logger.Errorf("[DEBUG] 执行磁盘使用计算命令失败: username=%s, homeDir=%s, err=%v, output=%q", username, homeDir, duErr, output)
	}

	sizeStr := strings.TrimSpace(output)
	var size int64
	if sizeStr != "" {
		fields := strings.Fields(sizeStr)
		if len(fields) > 0 {
			sizeStr = fields[0]
		}
		if sVal, parseErr := strconv.ParseInt(sizeStr, 10, 64); parseErr == nil {
			size = sVal
			logger.Infof("[DEBUG] 解析目录大小成功: size=%d bytes (%.2f MB)", size, float64(size)/1024/1024)
		} else {
			logger.Errorf("[DEBUG] 解析目录大小失败: sizeStr=%q, parseErr=%v", sizeStr, parseErr)
		}
	} else {
		logger.Warnf("[DEBUG] du命令输出为空，目录可能为空或无权限访问")
	}

	// 2. 获取磁盘总量（用于计算使用率）
	var totalBytes int64

	// 方法1：尝试使用 df 命令获取磁盘总量 - 不使用 2>/dev/null
	cmdDf := fmt.Sprintf("df -B1 %s | tail -n 1", safePath)
	logger.Infof("[DEBUG] 执行df命令: %s", cmdDf)
	outputDf2, exitCodeDf, errDf2 := session.Client.ExecuteCommandWithStatus(cmdDf)
	logger.Infof("[DEBUG] df命令结果: output=%q, exitCode=%d, err=%v", outputDf2, exitCodeDf, errDf2)

	outputDf := outputDf2
	var errDf error
	if errDf2 != nil && exitCodeDf == 0 {
		// 有错误但退出码为0，忽略
	} else if errDf2 != nil {
		errDf = errDf2
	}

	if errDf == nil && strings.TrimSpace(outputDf) != "" {
		logger.Infof("[DEBUG] df命令原始输出: %q", outputDf)
		fields := strings.Fields(strings.TrimSpace(outputDf))
		logger.Infof("[DEBUG] df输出字段数: %d, 字段内容: %v", len(fields), fields)
		if len(fields) >= 4 {
			// df 输出：Filesystem 1B-blocks Used Available Use% Mounted on
			if t, parseErr := strconv.ParseInt(fields[1], 10, 64); parseErr == nil && t > 0 {
				totalBytes = t
				logger.Infof("[DEBUG] 解析磁盘总量成功: totalBytes=%d bytes (%.2f GB)", totalBytes, float64(totalBytes)/1024/1024/1024)
			} else {
				logger.Errorf("[DEBUG] 解析df输出失败: fields[1]=%q, parseErr=%v", fields[1], parseErr)
			}
		} else {
			logger.Warnf("[DEBUG] df输出字段数不足: %d < 4", len(fields))
		}
	} else {
		logger.Warnf("[DEBUG] df命令执行失败或输出为空: errDf=%v, output=%q", errDf, outputDf)
	}

	// 方法2：如果 df 失败，尝试使用 stat -f
	if totalBytes == 0 {
		logger.Info("[DEBUG] df方法失败，尝试使用stat命令")
		cmdStat := fmt.Sprintf(`stat -f -c "%%b %%S" %s || stat -f "%%b %%S" %s`, safePath, safePath)
		logger.Infof("[DEBUG] 执行stat命令: %s", cmdStat)
		outStat2, exitCodeStat, errStat2 := session.Client.ExecuteCommandWithStatus(cmdStat)
		logger.Infof("[DEBUG] stat命令结果: output=%q, exitCode=%d, err=%v", outStat2, exitCodeStat, errStat2)

		outStat := outStat2
		var errStat error
		if errStat2 != nil && exitCodeStat == 0 {
			// 有错误但退出码为0，忽略
		} else if errStat2 != nil {
			errStat = errStat2
		}

		if errStat == nil && strings.TrimSpace(outStat) != "" {
			logger.Infof("[DEBUG] stat命令原始输出: %q", outStat)
			parts := strings.Fields(strings.TrimSpace(outStat))
			if len(parts) >= 2 {
				if b, errB := strconv.ParseInt(parts[0], 10, 64); errB == nil {
					if ssz, errS := strconv.ParseInt(parts[1], 10, 64); errS == nil {
						totalBytes = b * ssz
						logger.Infof("[DEBUG] 通过stat解析磁盘总量成功: blocks=%d, blockSize=%d, totalBytes=%d", b, ssz, totalBytes)
					} else {
						logger.Errorf("[DEBUG] 解析stat输出失败: parts[1]=%q, err=%v", parts[1], errS)
					}
				} else {
					logger.Errorf("[DEBUG] 解析stat输出失败: parts[0]=%q, err=%v", parts[0], errB)
				}
			} else {
				logger.Warnf("[DEBUG] stat输出字段数不足: %d < 2", len(parts))
			}
		} else {
			logger.Warnf("[DEBUG] stat命令执行失败或输出为空: errStat=%v, output=%q", errStat, outStat)
		}
	}

	// 3. 计算使用率
	var usage float64
	if totalBytes > 0 {
		usage = float64(size) / float64(totalBytes) * 100
		logger.Infof("[DEBUG] 计算磁盘使用率: size=%d, totalBytes=%d, usage=%.4f%%", size, totalBytes, usage)
	} else {
		logger.Warnf("[DEBUG] totalBytes为0，无法计算使用率")
	}

	logger.Infof("[DEBUG] 计算完成: username=%s, size=%d, totalBytes=%d, usage=%.4f%%", username, size, totalBytes, usage)
	return &dto.DiskUsageData{
		DirectorySize: size,
		DiskUsage:     usage,
	}, nil
}

// CalculateAllUsersDiskUsage 计算所有用户磁盘使用情况（定时任务调用）
func (s *fileService) CalculateAllUsersDiskUsage() error {
	if s.userRepo == nil {
		logger.Error("userRepo 未初始化，无法计算用户磁盘使用情况")
		return errors.New(errors.CodeInternalError, "userRepo 未初始化")
	}

	// 获取所有用户列表
	users, _, err := s.userRepo.List(0, 0, "", "", nil)
	if err != nil {
		logger.Error("获取用户列表失败", zap.Error(err))
		return errors.NewWithErr(errors.CodeInternalError, "获取用户列表失败", err)
	}

	if len(users) == 0 {
		logger.Info("没有用户需要计算磁盘使用情况")
		return nil
	}

	logger.Infof("开始计算 %d 个用户的磁盘使用情况", len(users))

	ctx := context.Background()
	basePath := config.Get().Server.BasePath
	if basePath == "" {
		basePath = "/home"
	}

	// 遍历每个用户，计算其主目录的磁盘使用情况
	for _, user := range users {
		if user.Username == "" {
			continue
		}

		// 构建用户主目录路径
		homeDir := path.Join(basePath, user.Username)

		logger.Infof("计算用户 %s (ID: %d) 的磁盘使用情况，路径: %s", user.Username, user.ID, homeDir)

		// 使用 root 会话计算磁盘使用情况（包括磁盘占比）
		diskUsageData, calcErr := s.calculateDiskUsageWithRoot(user.Username, homeDir)
		if calcErr != nil || diskUsageData == nil {
			logger.Warnf("计算用户磁盘使用情况失败: userID=%d, username=%s, err=%v", user.ID, user.Username, calcErr)
			continue
		}

		// 写入 Redis 缓存（使用统一的 key 格式）
		//cacheKey := fmt.Sprintf("disk:usage:%d:%s", user.ID, homeDir)
		if err := s.SetDiskUsageCache(ctx, int(user.ID), homeDir, diskUsageData); err != nil {
			logger.Warnf("写入磁盘使用缓存失败: userID=%d, err=%v", user.ID, err)
			continue
		}

		logger.Infof("用户 %s (ID: %d) 磁盘使用情况计算完成: size=%d bytes, usage=%.2f%%",
			user.Username, user.ID, diskUsageData.DirectorySize, diskUsageData.DiskUsage)
	}

	logger.Info("所有用户磁盘使用情况计算完成")
	return nil
}

// GetAllUsersDiskUsage 获取所有用户磁盘使用情况列表
func (s *fileService) GetAllUsersDiskUsage() ([]*dto.UserDiskUsageData, error) {
	logger.Info("[DEBUG] ========== 开始 GetAllUsersDiskUsage ==========")

	if s.userRepo == nil {
		logger.Error("[DEBUG] userRepo 未初始化，无法获取用户磁盘使用情况")
		return nil, errors.New(errors.CodeInternalError, "userRepo 未初始化")
	}

	// 获取所有用户列表（分页获取所有用户）
	users, total, err := s.userRepo.List(0, 0, "", "", nil)
	if err != nil {
		logger.Error("[DEBUG] 获取用户列表失败", zap.Error(err), zap.Int64("total", total))
		return nil, errors.NewWithErr(errors.CodeInternalError, "获取用户列表失败", err)
	}

	logger.Info("[DEBUG] 查询到的用户数量", zap.Int("count", len(users)), zap.Int64("total", total))
	logger.Infof("[DEBUG] 用户列表详情: %+v", users)

	if len(users) == 0 {
		logger.Info("[DEBUG] 没有用户，返回空列表")
		return []*dto.UserDiskUsageData{}, nil
	}

	ctx := context.Background()
	basePath := config.Get().Server.BasePath
	if basePath == "" {
		basePath = "/home"
	}
	logger.Infof("[DEBUG] 基础路径: %s", basePath)

	var result []*dto.UserDiskUsageData

	for _, user := range users {
		if user.Username == "" {
			logger.Infof("[DEBUG] 跳过空用户名: userID=%d", user.ID)
			continue
		}

		homeDir := path.Join(basePath, user.Username)
		cacheKey := fmt.Sprintf("disk:usage:%d:%s", user.ID, homeDir)
		logger.Infof("[DEBUG] ========== 处理用户: userID=%d, username=%s, homeDir=%s ==========", user.ID, user.Username, homeDir)

		// 从 Redis 获取缓存数据
		logger.Infof("[DEBUG] 尝试从Redis读取缓存: key=%s", cacheKey)
		cacheData, err := s.GetDiskUsageCache(ctx, int(user.ID), homeDir)
		if err != nil {
			logger.Warnf("[DEBUG] 读取Redis缓存失败: userID=%d, username=%s, key=%s, err=%v",
				user.ID, user.Username, cacheKey, err)
		}

		if err != nil || cacheData == nil {
			// 缓存不存在，使用 root 会话直接计算（避免为每个用户建立 SSH 会话）
			logger.Infof("[DEBUG] 缓存不存在或为空，开始计算磁盘使用: userID=%d, username=%s, homeDir=%s",
				user.ID, user.Username, homeDir)
			diskUsageData, calcErr := s.calculateDiskUsageWithRoot(user.Username, homeDir)
			if calcErr != nil || diskUsageData == nil {
				// 计算也失败，返回0
				logger.Errorf("[DEBUG] !!! 计算失败，返回0值: userID=%d, username=%s, calcErr=%v, diskUsageData=%v",
					user.ID, user.Username, calcErr, diskUsageData)
				result = append(result, &dto.UserDiskUsageData{
					UserID:        int64(user.ID),
					Username:      user.Username,
					DirectorySize: 0,
					DiskUsage:     0,
					HomeDirectory: homeDir,
				})
				logger.Warnf("[DEBUG] 获取用户磁盘使用情况失败: userID=%d, username=%s, homeDir=%s, err=%v",
					user.ID, user.Username, homeDir, calcErr)
				continue
			}

			logger.Infof("[DEBUG] !!! 计算成功: userID=%d, username=%s, size=%d (%.2f MB), usage=%.4f%%",
				user.ID, user.Username, diskUsageData.DirectorySize, float64(diskUsageData.DirectorySize)/1024/1024, diskUsageData.DiskUsage)

			// 写入 Redis 缓存，避免下次重复计算
			if err := s.SetDiskUsageCache(ctx, int(user.ID), homeDir, diskUsageData); err != nil {
				logger.Warnf("[DEBUG] 写入磁盘使用缓存失败: userID=%d, username=%s, err=%v",
					user.ID, user.Username, err)
			} else {
				logger.Infof("[DEBUG] 成功写入Redis缓存: userID=%d, username=%s", user.ID, user.Username)
			}

			result = append(result, &dto.UserDiskUsageData{
				UserID:        int64(user.ID),
				Username:      user.Username,
				DirectorySize: diskUsageData.DirectorySize,
				DiskUsage:     diskUsageData.DiskUsage,
				HomeDirectory: homeDir,
			})
			continue
		}

		logger.Infof("[DEBUG] 使用缓存数据: userID=%d, username=%s, size=%d (%.2f MB), usage=%.4f%%",
			user.ID, user.Username, cacheData.DirectorySize, float64(cacheData.DirectorySize)/1024/1024, cacheData.DiskUsage)

		result = append(result, &dto.UserDiskUsageData{
			UserID:        int64(user.ID),
			Username:      user.Username,
			DirectorySize: cacheData.DirectorySize,
			DiskUsage:     cacheData.DiskUsage,
			HomeDirectory: homeDir,
		})
	}

	logger.Infof("[DEBUG] ========== GetAllUsersDiskUsage 完成，返回 %d 条记录 ==========", len(result))
	for i, data := range result {
		logger.Infof("[DEBUG] 结果[%d]: userID=%d, username=%s, size=%d, usage=%.4f%%",
			i, data.UserID, data.Username, data.DirectorySize, data.DiskUsage)
	}

	return result, nil
}
