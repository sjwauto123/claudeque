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
		return nil, err
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
	targetPath = strings.TrimSpace(targetPath)
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

	// 统一路径格式，确保以 / 结尾，保证 Redis Key 一致性
	// 用户期望统一为 /home/admin/ 这种格式
	keyPath := resolvedPath
	if !strings.HasSuffix(keyPath, "/") {
		keyPath += "/"
	}

	// 检查是否有正在进行的上传任务 (简单的锁机制)
	lockKey := fmt.Sprintf("upload:lock:%d", userID)
	if s.redisRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		// 尝试获取锁，如果存在则说明有任务在运行
		val, err := s.redisRepo.Get(ctx, lockKey)
		cancel()
		if err == nil && val != "" && val != safeFilename {
			return nil, errors.New(errors.CodeInvalidParam, fmt.Sprintf("当前有正在进行的上传任务: %s，请等待完成后再试", val))
		}
		// 设置锁，有效期 1 小时 (防止死锁)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
		_ = s.redisRepo.Set(ctx2, lockKey, safeFilename, 1*time.Hour)
		cancel2()
	}

	remotePath := path.Join(resolvedPath, safeFilename)
	// 因为改为异步，这里不再创建 dst，而是在 goroutine 里创建
	// dst, err := client.Create(remotePath)
	// if err != nil {
	// 	logger.Errorf("创建远程文件失败: userID=%d, remotePath=%s, err=%v", userID, remotePath, err)
	// 	return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("创建远程文件失败: %s", remotePath), err)
	// }
	// defer func(dst *sftp.File) {
	// 	err := dst.Close()
	// 	if err != nil {
	// 		logger.Errorf("关闭远程文件失败: userID=%d, remotePath=%s, err=%v", userID, remotePath, err)
	// 	}
	// }(dst)

	// 进度上报
	var total int64 = header.Size
	// 使用 filename + targetPath 作为唯一标识，防止不同目录下同名文件进度冲突
	progressKey := fmt.Sprintf("upload:progress:%d:%s:%s", userID, safeFilename, keyPath)

	// Debug Log
	logger.Infof("UploadFile: progressKey=%s, userID=%d, filename=%s, targetPath=%s, resolvedPath=%s, keyPath=%s, isRootMode=%v",
		progressKey, userID, header.Filename, targetPath, resolvedPath, keyPath, isRootMode)

	startTime := time.Now()

	report := func(sent int64, status string, errMsg string) {
		if s.redisRepo == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var percent float64
		if total > 0 {
			percent = float64(sent) / float64(total) * 100
			if percent > 100 {
				percent = 100
			}
		}

		// 计算速度和剩余时间
		duration := time.Since(startTime).Seconds()
		var speed string
		var remaining string
		if duration > 0 && sent > 0 {
			bytesPerSec := float64(sent) / duration
			speed = formatUploadSpeed(bytesPerSec)
			if total > sent {
				remBytes := float64(total - sent)
				remSec := remBytes / bytesPerSec
				remaining = formatUploadDuration(time.Duration(remSec) * time.Second)
			}
		} else if status == "uploading" {
			// 初始化状态，速度和时间为 0，防止只有名字和大小
			speed = "0 B/s"
			remaining = "计算中..."
		}

		data := &dto.UploadProgressData{
			Filename:  safeFilename,
			Status:    status,
			Progress:  percent,
			Uploaded:  sent,
			Total:     total,
			Speed:     speed,
			Remaining: remaining,
			Error:     errMsg,
		}

		if b, e := json.Marshal(data); e == nil {
			_ = s.redisRepo.Set(ctx, progressKey, string(b), 30*time.Minute)
		}
	}
	// 初始状态上报
	report(0, "uploading", "")

	// 优化 buffer 大小：128KB 较适合 SFTP 的包对齐

	// 使用协程异步处理大文件上传，避免阻塞 HTTP 响应
	// 方案 B：先存临时文件，再异步上传

	// 创建临时文件
	tempFile, err := os.CreateTemp("", fmt.Sprintf("upload_%d_%s_*", userID, safeFilename))
	if err != nil {
		logger.Errorf("创建临时文件失败: %v", err)
		return nil, errors.New(errors.CodeInternalError, "创建临时文件失败")
	}
	tempPath := tempFile.Name()

	// 将 multipart 文件写入临时文件
	_, err = io.Copy(tempFile, file)
	if err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		logger.Errorf("写入临时文件失败: %v", err)
		return nil, errors.New(errors.CodeInternalError, "写入临时文件失败")
	}
	tempFile.Close()

	// 启动异步上传任务
	go func() {
		defer os.Remove(tempPath)
		defer func() {
			// 释放锁
			if s.redisRepo != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = s.redisRepo.Del(ctx, lockKey)
				cancel()
			}
		}()

		// 重新打开临时文件
		localFile, err := os.Open(tempPath)
		if err != nil {
			report(0, "failed", "读取临时文件失败")
			return
		}
		defer localFile.Close()

		// 重新获取 SFTP 客户端 (避免闭包捕获的 client 失效)
		asyncClient, err := s.getSftpClient(userID, isRootMode)
		if err != nil {
			report(0, "failed", "SFTP连接失败")
			return
		}

		asyncDst, err := asyncClient.Create(remotePath)
		if err != nil {
			report(0, "failed", fmt.Sprintf("创建远程文件失败: %v", err))
			return
		}
		defer asyncDst.Close()

		// 开始传输
		buf := make([]byte, 128*1024)
		var sent int64 = 0
		lastReportTime := time.Now()
		lastReportSent := int64(0)

		for {
			nr, rerr := localFile.Read(buf)
			if nr > 0 {
				nw, werr := asyncDst.Write(buf[:nr])
				if werr != nil {
					report(sent, "failed", werr.Error())
					return
				}
				sent += int64(nw)

				// 频率控制
				if time.Since(lastReportTime) > 500*time.Millisecond || (sent-lastReportSent) > 2*1024*1024 {
					report(sent, "uploading", "")
					lastReportTime = time.Now()
					lastReportSent = sent
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				report(sent, "failed", rerr.Error())
				return
			}
		}

		report(sent, "done", "")
		s.invalidateFileListCache(userID, isRootMode, resolvedPath)
	}()

	// 主线程不再执行上传循环，直接返回成功
	// 注意：因为改为异步，主线程不应该释放锁（锁在 goroutine 里释放）
	// 但是上面的代码里 lockKey 的 defer delete 是在 if s.redisRepo != nil 代码块里定义的吗？
	// 检查代码发现：defer func() { ... s.redisRepo.Del(ctx, lockKey) ... }() 是在 if 块里定义的。
	// 这意味着主线程退出时会执行这个 defer，导致锁被释放。
	// 我们需要修改锁的逻辑：不使用 defer 释放锁，而是手动释放。

	// 由于 SearchReplace 只能替换局部，我们先还原上面的代码结构，再做修改。
	// 这是一个较大的逻辑变更，我将分两步操作：
	// 1. 先用 Read 读取完整的 UploadFile 函数。
	// 2. 用 Write 重写整个 UploadFile 函数。

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

// GetDiskUsage 计算目录大小及磁盘占用
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
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", safePath)
	output, err := s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		// 尝试回退方案
		outStr := strings.TrimSpace(output)
		if strings.Contains(err.Error(), "Permission denied") || strings.Contains(outStr, "Permission denied") {
			errDu = errors.NewWithErr(errors.CodeForbidden, "权限不足，无法访问该目录", err)
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
				}
			} else {
				errDu = errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, fmt.Sprintf("获取目录大小失败: %v, output: %s", err, output), err)
			}
		}
	} else {
		sizeStr := strings.TrimSpace(output)
		if sizeStr == "" {
			// 如果 output 为空但 err 为 nil，可能是管道问题，尝试直接 du -sb
			cmdDirect := fmt.Sprintf("du -sb %s", safePath)
			outDirect, errDirect := s.executeSSHCommand(userID, cmdDirect, isRootMode)
			if errDirect == nil && strings.TrimSpace(outDirect) != "" {
				sizeStr = strings.TrimSpace(outDirect)
			} else {
				size = 0
				logger.Warnf("du 命令返回空输出: userID=%d, path=%s", userID, safePath)
			}
		}

		if sizeStr != "" {
			fields := strings.Fields(sizeStr)
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
		return nil, errors.New(errors.CodeInternalError, "未配置进度存储")
	}

	filename = strings.TrimSpace(filename)
	targetPath = strings.TrimSpace(targetPath)

	// 1. 获取用户信息，判断是否是 root 模式（与上传逻辑保持一致）
	isRootMode, err := s.authService.HasSystemAccess(userID, AccessTypeFile)
	if err != nil {
		return nil, err
	}

	// 2. 解析路径以获取 resolvedPath，确保与上传时生成的 Key 一致
	resolvedPath, err := s.resolvePath(userID, targetPath, isRootMode)
	if err != nil {
		return nil, err
	}

	// 统一处理文件名，移除可能的路径前缀
	safeFilename := path.Base(filename)

	// 统一路径格式，确保以 / 结尾，保证 Redis Key 一致性
	keyPath := resolvedPath
	if !strings.HasSuffix(keyPath, "/") {
		keyPath += "/"
	}

	key := fmt.Sprintf("upload:progress:%d:%s:%s", userID, safeFilename, keyPath)

	// Debug Log
	logger.Infof("GetUploadProgress: key=%s, userID=%d, filename=%s, targetPath=%s, resolvedPath=%s, keyPath=%s, isRootMode=%v",
		key, userID, filename, targetPath, resolvedPath, keyPath, isRootMode)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	val, err := s.redisRepo.Get(ctx, key)
	if err != nil || val == "" {
		return &dto.UploadProgressData{
			Filename: safeFilename,
			Status:   "unknown",
		}, nil
	}
	var out dto.UploadProgressData
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalError, "解析进度失败", err)
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
