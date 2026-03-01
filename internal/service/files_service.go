package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/pkg/config"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/pkg/sftp"
)

// fileService 文件服务实现
type fileService struct {
	sessionManager *ssh.SessionManager
	authService    AuthService
}

// NewFileService 创建文件服务
func NewFileService(sessionManager *ssh.SessionManager, authService AuthService) FileService {
	return &fileService{
		sessionManager: sessionManager,
		authService:    authService,
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
		// 尝试轻量级 ping
		if _, pingErr := session.Client.ExecuteCommand("echo 1"); pingErr == nil {
			// 确保 SFTP 客户端也存在
			if session.Client.GetSFTPClient() != nil {
				isValid = true
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
	sftpClient := session.Client.GetSFTPClient()
	_, err = sftpClient.Stat(homeDir)
	if err != nil && os.IsNotExist(err) {
		logger.Infof("用户主目录不存在，尝试创建: userID=%d, homeDir=%s", userID, homeDir)
		if err = session.Client.MkdirAll(homeDir); err != nil {
			logger.Errorf("创建用户主目录失败: userID=%d, homeDir=%s, err=%v", userID, homeDir, err)
			return "", errors.NewWithErr(errors.CodeInternalError, "创建用户主目录失败", err)
		}
		logger.Infof("用户主目录创建成功: userID=%d, homeDir=%s", userID, homeDir)
	} else if err != nil {
		logger.Errorf("检查用户主目录状态失败: userID=%d, homeDir=%s, err=%v", userID, homeDir, err)
		return "", errors.NewWithErr(errors.CodeInternalError, "检查用户主目录状态失败", err)
	}

	//防止所传path为空的情况
	if p == "" || p == "." || p == "/" {
		return homeDir, nil
	}

	// 如果路径已经是绝对路径且不在家目录下，或者包含 .. 试图越权，则纠正或拒绝
	//先进行标准化处理，再判断是否满足条件
	cleanPath := path.Clean(p)
	if path.IsAbs(cleanPath) {
		if strings.HasPrefix(cleanPath, homeDir) {
			return cleanPath, nil
		}
		// 如果是绝对路径但不以 homeDir 开头，则重新拼接到 homeDir 下
		// 去掉前导斜杠
		relPath := strings.TrimPrefix(cleanPath, "/")
		return path.Join(homeDir, relPath), nil
	}

	// 拼接到家目录下
	finalPath := path.Join(homeDir, cleanPath)
	//二次确认
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
	//启动对应的ftpclient
	client, err := s.getSftpClient(opUserID, isRootMode)
	if err != nil {
		logger.Errorf("获取SFTP客户端失败: userID=%d, isRootMode=%t, err=%v", opUserID, isRootMode, err)
		return nil, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	//解析出权限正确的路径
	resolvedPath, err := s.resolvePath(opUserID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("解析路径失败: userID=%d, path=%s, isRootMode=%t, err=%v", opUserID, req.Path, isRootMode, err)
		return nil, err
	}

	//传化为真实可用的路径
	realPath, err := client.RealPath(resolvedPath)
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

	var dirs []*dto.DirectoryItem
	var files []*dto.FileItem

	// 构建 UID -> Username 映射
	//获取对应系统中文件的真正用户名，而不是uid，用来返回文件所有者
	uidToName := make(map[uint32]string)

	passwdOut, err := s.executeSSHCommand(opUserID, "getent passwd || cat /etc/passwd", isRootMode)
	if err != nil {
		logger.Errorf("执行SSH命令获取passwd失败: userID=%d, err=%v", opUserID, err)
	} else {
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

	//
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })

	// Pagination
	totalDirs := len(dirs)
	totalFiles := len(files)
	total := totalDirs + totalFiles
	pages := total / req.PageSize
	if total%req.PageSize != 0 {
		pages++
	}

	start := (req.Page - 1) * req.PageSize

	if start >= total {
		dirs = []*dto.DirectoryItem{}
		files = []*dto.FileItem{}
	} else {
		var slicedDirs []*dto.DirectoryItem
		var slicedFiles []*dto.FileItem

		currentIdx := 0
		count := 0

		// Add dirs
		for _, d := range dirs {
			if currentIdx >= start && count < req.PageSize {
				slicedDirs = append(slicedDirs, d)
				count++
			}
			currentIdx++
		}

		// Add files
		for _, f := range files {
			if currentIdx >= start && count < req.PageSize {
				slicedFiles = append(slicedFiles, f)
				count++
			}
			currentIdx++
		}

		dirs = slicedDirs
		files = slicedFiles
	}

	// Build Breadcrumb
	breadcrumb := buildBreadcrumb(realPath)

	return &dto.FilesListData{
		Breadcrumb:    breadcrumb,
		Path:          realPath,
		FileList:      files,
		DirectoryList: dirs,
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

	remotePath := path.Join(resolvedPath, header.Filename)
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

	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"
	safeTarget := "'" + strings.ReplaceAll(resolvedTarget, "'", "'\\''") + "'"

	cmd := fmt.Sprintf("unzip -o %s -d %s", safePath, safeTarget)
	_, err = s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		logger.Errorf("执行解压命令失败: userID=%d, cmd=%s, err=%v", userID, cmd, err)
		return errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, "执行解压命令失败", err)
	}
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
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", safePath)

	output, err := s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		logger.Errorf("执行du命令失败: userID=%d, cmd=%s, err=%v", userID, cmd, err)
		// 解析错误信息，如果是 Permission denied，则返回更友好的提示
		errMsg := err.Error()
		if strings.Contains(errMsg, "Permission denied") {
			return nil, errors.NewWithErr(errors.CodeForbidden, "权限不足，无法访问该目录", err)
		}
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
			kb, parseErr := strconv.ParseInt(kbStr, 10, 64)
			if parseErr == nil {
				output = fmt.Sprintf("%d", kb*1024)
			}
		} else {
			return nil, errors.NewWithErr(errors.CodeSSHCommandExecutionFailed, fmt.Sprintf("获取目录大小失败: %v", err), err)
		}
	}

	sizeStr := strings.TrimSpace(output)
	if sizeStr == "" {
		logger.Warnf("du命令返回为空: userID=%d, cmd=%s", userID, cmd)
		sizeStr = "0"
	}

	// 修复：如果 du -sb 输出包含文件名，只取第一列
	fields := strings.Fields(sizeStr)
	if len(fields) > 0 {
		sizeStr = fields[0]
	}
	size, parseErr := strconv.ParseInt(sizeStr, 10, 64)
	if parseErr != nil {
		logger.Errorf("解析目录大小失败: userID=%d, sizeStr=%s, err=%v", userID, sizeStr, parseErr)
		return nil, errors.NewWithErr(errors.CodeInternalError, fmt.Sprintf("解析目录大小数据失败: %s", sizeStr), parseErr)
	}

	// 2. 使用 df 获取磁盘总大小，计算占比
	// 使用 df -B1 获取以字节为单位的总大小
	cmdTotal := fmt.Sprintf("df -B1 %s | tail -1 | awk '{print $2}'", safePath)
	outTotal, err := s.executeSSHCommand(userID, cmdTotal, isRootMode)

	var usage float64
	if err == nil {
		totalStr := strings.TrimSpace(outTotal)
		totalBytes, parseErr := strconv.ParseInt(totalStr, 10, 64)
		if parseErr == nil && totalBytes > 0 {
			usage = float64(size) / float64(totalBytes) * 100
		} else {
			logger.Errorf("解析磁盘总大小失败或总大小为0: userID=%d, totalStr=%s, err=%v", userID, totalStr, parseErr)
		}
	} else {
		logger.Errorf("执行df命令获取总量失败: userID=%d, cmd=%s, err=%v", userID, cmdTotal, err)
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
	cmdTotal := fmt.Sprintf("df -B1 %s | tail -1 | awk '{print $2}'", quotedPath)
	outTotal, err := s.executeSSHCommand(userID, cmdTotal, isRootMode)

	var usagePercent float64
	if err == nil {
		totalStr := strings.TrimSpace(outTotal)
		totalBytes, _ := strconv.ParseInt(totalStr, 10, 64)

		if totalBytes > 0 {
			usagePercent = float64(bytess) / float64(totalBytes) * 100
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
