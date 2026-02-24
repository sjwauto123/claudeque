package service

import (
	"bytes"
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/pkg/errors"
	"cloudque/pkg/ssh"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	xssh "golang.org/x/crypto/ssh"
)

// FileService 文件服务接口
type FileService interface {
	GetFileList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error)
	UploadFile(userID int, file multipart.File, header *multipart.FileHeader, targetPath string, isRootMode bool) (*dto.FileUploadData, error)
	DownloadFile(userID int, path string, isRootMode bool) (io.ReadCloser, string, error)
	DeleteFile(userID int, path string, isRootMode bool) error
	UnzipFile(userID int, req *request.UnzipRequest, isRootMode bool) error
	GetDiskUsage(userID int, path string, isRootMode bool) (*dto.DiskUsageData, error)
	CalculateSize(userID int, path string, isRootMode bool) (int64, string, float64, error)
}

// fileService 文件服务实现
type fileService struct {
	sessionManager *ssh.SessionManager
}

// NewFileService 创建文件服务
func NewFileService(sessionManager *ssh.SessionManager) FileService {
	return &fileService{
		sessionManager: sessionManager,
	}
}

// getSftpClient 获取用户的SFTP客户端
func (s *fileService) getSftpClient(userID int, isRoot bool) (*sftp.Client, error) {
	if s.sessionManager == nil {
		return nil, errors.New(errors.CodeInternalError, "SSH会话管理器未初始化")
	}

	session, err := s.sessionManager.GetSession(userID, isRoot)
	if err != nil {
		return nil, errors.New(errors.CodeUnauthorized, "SSH会话已过期，请重新登录")
	}

	if session.Client == nil {
		return nil, errors.New(errors.CodeInternalError, "SSH客户端未连接")
	}

	return session.Client.GetSFTPClient(), nil
}

// getSSHClient 获取用户的SSH客户端
func (s *fileService) getSSHClient(userID int, isRoot bool) (*xssh.Client, error) {
	if s.sessionManager == nil {
		return nil, errors.New(errors.CodeInternalError, "SSH会话管理器未初始化")
	}
	session, err := s.sessionManager.GetSession(userID, isRoot)
	if err != nil {
		return nil, errors.New(errors.CodeUnauthorized, "SSH会话已过期，请重新登录")
	}
	return session.Client.GetSSHClient(), nil
}

// executeSSHCommand 执行SSH命令
func (s *fileService) executeSSHCommand(userID int, cmd string, isRoot bool) (string, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("执行命令失败: %s, 错误: %v", stderr.String(), err)
	}

	return stdout.String(), nil
}

// resolvePath 解析路径并应用权限隔离
func (s *fileService) resolvePath(userID int, path string, isRootMode bool) (string, error) {
	session, err := s.sessionManager.GetSession(userID, isRootMode)
	if err != nil {
		return "", err
	}
	//
	//管理员模式:只需要处理为空的情况，不需要验证路径直接返回
	if isRootMode {
		//防止所传path为空的情况
		if path == "" || path == "." {
			return "/", nil
		}
		return path, nil
	}

	// 用户模式：限制在 /home/{username}
	homeDir := "/home/" + session.Username
	//防止所传path为空的情况
	if path == "" || path == "." || path == "/" {
		return homeDir, nil
	}

	// 如果路径已经是绝对路径且不在家目录下，或者包含 .. 试图越权，则纠正或拒绝
	//先进行标准化处理，再判断是否满足条件
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, homeDir) {
		return cleanPath, nil
	}

	// 拼接到家目录下
	finalPath := filepath.ToSlash(filepath.Join(homeDir, path))
	//二次确认
	if !strings.HasPrefix(finalPath, homeDir) {
		return "", fmt.Errorf("越权访问被拒绝")
	}

	return finalPath, nil
}

// GetFileList 获取文件列表
func (s *fileService) GetFileList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error) {
	opUserID := userID

	//启动对应的ftpclient
	client, err := s.getSftpClient(opUserID, isRootMode)
	if err != nil {
		return nil, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	//解析出权限正确的路径
	path, err := s.resolvePath(opUserID, req.Path, isRootMode)
	if err != nil {
		return nil, err
	}

	//传化为真实可用的路径
	realPath, err := client.RealPath(path)
	if err != nil {
		return nil, fmt.Errorf("无法解析路径: %v", err)
	}

	//读取目标目录下的所有文件和目录
	entries, err := client.ReadDir(realPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	var dirs []*dto.DirectoryItem
	var files []*dto.FileItem

	// 构建 UID -> Username 映射
	//获取对应系统中文件的真正用户名，而不是uid，用来返回文件所有者
	uidToName := make(map[uint32]string)

	passwdOut, err := s.executeSSHCommand(opUserID, "getent passwd || cat /etc/passwd", isRootMode)
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
				Path:      filepath.ToSlash(filepath.Join(realPath, entry.Name())),
				Owner:     owner,
			})
		} else {
			files = append(files, &dto.FileItem{
				Filename:  entry.Name(),
				FileSize:  entry.Size(),
				UpdatedAt: entry.ModTime(),
				Path:      filepath.ToSlash(filepath.Join(realPath, entry.Name())),
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
		slicedDirs := []*dto.DirectoryItem{}
		slicedFiles := []*dto.FileItem{}

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

func buildBreadcrumb(path string) []*dto.BreadcrumbItem {
	parts := strings.Split(path, "/")
	var items []*dto.BreadcrumbItem

	// Always add Root for absolute paths
	if strings.HasPrefix(path, "/") {
		items = append(items, &dto.BreadcrumbItem{Name: "/", Path: "/"})
	}

	currentPath := ""
	for _, p := range parts {
		if p == "" {
			if currentPath == "" {
				currentPath = "/"
			}
			continue
		}
		if currentPath == "/" {
			currentPath += p
		} else {
			currentPath += "/" + p
		}
		items = append(items, &dto.BreadcrumbItem{
			Name: p,
			Path: currentPath,
		})
	}
	return items
}

// UploadFile 上传文件
func (s *fileService) UploadFile(userID int, file multipart.File, header *multipart.FileHeader, targetPath string, isRootMode bool) (*dto.FileUploadData, error) {
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		return nil, err
	}

	path, err := s.resolvePath(userID, targetPath, isRootMode)
	if err != nil {
		return nil, err
	}

	// Ensure target directory exists
	_ = client.MkdirAll(path)

	remotePath := filepath.ToSlash(filepath.Join(path, header.Filename))
	dst, err := client.Create(remotePath)
	if err != nil {
		return nil, fmt.Errorf("创建远程文件失败: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return nil, fmt.Errorf("写入文件失败: %v", err)
	}

	return &dto.FileUploadData{
		Filename: header.Filename,
	}, nil
}

// DownloadFile 下载文件
func (s *fileService) DownloadFile(userID int, path string, isRootMode bool) (io.ReadCloser, string, error) {
	client, err := s.getSftpClient(userID, isRootMode)
	if err != nil {
		return nil, "", err
	}

	resolvedPath, err := s.resolvePath(userID, path, isRootMode)
	if err != nil {
		return nil, "", err
	}

	f, err := client.Open(resolvedPath)
	if err != nil {
		return nil, "", fmt.Errorf("打开文件失败: %v", err)
	}

	return f, filepath.Base(resolvedPath), nil
}

// DeleteFile 删除文件或目录
func (s *fileService) DeleteFile(userID int, path string, isRootMode bool) error {
	resolvedPath, err := s.resolvePath(userID, path, isRootMode)
	if err != nil {
		return err
	}

	// 使用 rm -rf 强制删除，支持文件和目录
	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"
	cmd := fmt.Sprintf("rm -rf %s", safePath)

	_, err = s.executeSSHCommand(userID, cmd, isRootMode)
	return err
}

// UnzipFile 解压文件
func (s *fileService) UnzipFile(userID int, req *request.UnzipRequest, isRootMode bool) error {
	resolvedPath, err := s.resolvePath(userID, req.Path, isRootMode)
	if err != nil {
		return err
	}
	resolvedTarget, err := s.resolvePath(userID, req.TargetPath, isRootMode)
	if err != nil {
		return err
	}

	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"
	safeTarget := "'" + strings.ReplaceAll(resolvedTarget, "'", "'\\''") + "'"

	cmd := fmt.Sprintf("unzip -o %s -d %s", safePath, safeTarget)
	_, err = s.executeSSHCommand(userID, cmd, isRootMode)
	return err
}

// GetDiskUsage 计算目录大小
func (s *fileService) GetDiskUsage(userID int, path string, isRootMode bool) (*dto.DiskUsageData, error) {
	resolvedPath, err := s.resolvePath(userID, path, isRootMode)
	if err != nil {
		return nil, err
	}

	safePath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", safePath)

	output, err := s.executeSSHCommand(userID, cmd, isRootMode)
	if err != nil {
		return nil, err
	}

	sizeStr := strings.TrimSpace(output)
	size, _ := strconv.ParseInt(sizeStr, 10, 64)

	// Get filesystem usage percentage
	cmdUsage := fmt.Sprintf("df --output=pcent %s | tail -1 | tr -dc '0-9'", safePath)
	outputUsage, _ := s.executeSSHCommand(userID, cmdUsage, isRootMode)
	usage, _ := strconv.ParseFloat(strings.TrimSpace(outputUsage), 64)

	return &dto.DiskUsageData{
		DirectorySize: size,
		DiskUsage:     usage,
	}, nil
}

// CalculateSize 计算目录或文件大小
func (s *fileService) CalculateSize(userID int, path string, isRootMode bool) (int64, string, float64, error) {
	resolvedPath, err := s.resolvePath(userID, path, isRootMode)
	if err != nil {
		return 0, "", 0, err
	}

	quotedPath := "'" + strings.ReplaceAll(resolvedPath, "'", "'\\''") + "'"

	// 1. 使用 du -sb 计算字节大小
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", quotedPath)
	out, err := s.executeSSHCommand(userID, cmd, isRootMode)

	var bytes int64

	if err != nil {
		// 如果 du -sb 失败，尝试 du -sk (千字节)
		cmd = fmt.Sprintf("du -sk %s | awk '{print $1}'", quotedPath)
		out, err = s.executeSSHCommand(userID, cmd, isRootMode)
		if err != nil {
			return 0, "", 0, err
		}
		// 转换为字节
		kbStr := strings.TrimSpace(out)
		kb, parseErr := strconv.ParseInt(kbStr, 10, 64)
		if parseErr != nil {
			return 0, "", 0, fmt.Errorf("解析大小失败: %v", parseErr)
		}
		bytes = kb * 1024
	} else {
		sizeStr := strings.TrimSpace(out)
		bytes, err = strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return 0, "", 0, fmt.Errorf("解析大小失败: %v", err)
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
			usagePercent = float64(bytes) / float64(totalBytes) * 100
		}
	}

	return bytes, formatFileSize(bytes), usagePercent, nil
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
