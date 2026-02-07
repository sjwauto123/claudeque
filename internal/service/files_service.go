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
	GetFileList(userID uint, req *request.FileListRequest) (*dto.FilesListData, error)
	UploadFile(userID uint, file multipart.File, header *multipart.FileHeader, targetPath string) (*dto.FileUploadData, error)
	DownloadFile(userID uint, path string) (io.ReadCloser, string, error)
	DeleteFile(userID uint, path string) error
	UnzipFile(userID uint, req *request.UnzipRequest) error
	GetDiskUsage(userID uint, path string) (*dto.DiskUsageData, error)
	Chmod(userID uint, req *request.ChmodRequest) error
	CalculateSize(userID uint, path string) (int64, string, float64, error)
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
func (s *fileService) getSftpClient(userID uint) (*sftp.Client, error) {
	if s.sessionManager == nil {
		return nil, errors.New(errors.CodeInternalError, "SSH会话管理器未初始化")
	}

	session, err := s.sessionManager.GetSession(userID)
	if err != nil {
		return nil, errors.New(errors.CodeUnauthorized, "SSH会话已过期，请重新登录")
	}

	if session.Client == nil {
		return nil, errors.New(errors.CodeInternalError, "SSH客户端未连接")
	}

	return session.Client.GetSFTPClient(), nil
}

// getSSHClient 获取用户的SSH客户端
func (s *fileService) getSSHClient(userID uint) (*xssh.Client, error) {
	if s.sessionManager == nil {
		return nil, errors.New(errors.CodeInternalError, "SSH会话管理器未初始化")
	}
	session, err := s.sessionManager.GetSession(userID)
	if err != nil {
		return nil, errors.New(errors.CodeUnauthorized, "SSH会话已过期，请重新登录")
	}
	return session.Client.GetSSHClient(), nil
}

// executeSSHCommand 执行SSH命令
func (s *fileService) executeSSHCommand(userID uint, cmd string) (string, error) {
	client, err := s.getSSHClient(userID)
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

// GetFileList 获取文件列表
func (s *fileService) GetFileList(userID uint, req *request.FileListRequest) (*dto.FilesListData, error) {
	// 确定实际要操作的用户 ID
	opUserID := userID
	if req.TargetUserID > 0 {
		// 管理员权限检查逻辑：
		// 1. 获取当前用户
		// 2. 检查角色是否为管理员
		// 这里为了保持逻辑内聚，我们通过 sessionManager 获取当前用户的 session 来判断
		currentSession, err := s.sessionManager.GetSession(userID)
		if err != nil || !currentSession.IsRoot() {
			// 如果当前用户不是 root 且没有管理员标识，拒绝访问他人目录
			// 注意：这里 IsRoot() 是判断 SSH 连接是否为 root，
			// 在实际业务中，可能还需要检查数据库里的 User.RoleID
			return nil, errors.New(errors.CodeForbidden, "没有权限查看其他用户的目录")
		}
		opUserID = req.TargetUserID
	}

	client, err := s.getSftpClient(opUserID)
	if err != nil {
		return nil, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}

	path := req.Path
	if path == "" || path == "." {
		// 检查是否为 root 用户，如果是则默认从 / 开始
		session, err := s.sessionManager.GetSession(opUserID)
		if err == nil && session.IsRoot() {
			path = "/"
		} else if path == "" {
			path = "." // Default to home/current
		}
	}

	// Resolve real path to return in breadcrumb
	realPath, err := client.RealPath(path)
	if err != nil {
		return nil, fmt.Errorf("无法解析路径: %v", err)
	}

	entries, err := client.ReadDir(realPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	// Filter and separate
	var dirs []*dto.DirectoryItem
	var files []*dto.FileItem

	// Build UID -> Username map
	uidToName := make(map[uint32]string)

	// Fetch /etc/passwd to map UIDs to names
	// Try getent first, then cat /etc/passwd
	passwdOut, err := s.executeSSHCommand(opUserID, "getent passwd || cat /etc/passwd")
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
		// Skip hidden files if needed? keeping them for now.
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
				// Fallback if Sys() is not *sftp.FileStat (unlikely for sftp client)
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

	// Sort: Dirs first (alphabetical), then Files (alphabetical)
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })

	// Pagination
	totalDirs := len(dirs)
	totalFiles := len(files)
	total := totalDirs + totalFiles

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
func (s *fileService) UploadFile(userID uint, file multipart.File, header *multipart.FileHeader, targetPath string) (*dto.FileUploadData, error) {
	client, err := s.getSftpClient(userID)
	if err != nil {
		return nil, err
	}

	// Ensure target directory exists
	_ = client.MkdirAll(targetPath)

	remotePath := filepath.ToSlash(filepath.Join(targetPath, header.Filename))
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
func (s *fileService) DownloadFile(userID uint, path string) (io.ReadCloser, string, error) {
	client, err := s.getSftpClient(userID)
	if err != nil {
		return nil, "", err
	}

	f, err := client.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("打开文件失败: %v", err)
	}

	return f, filepath.Base(path), nil
}

// DeleteFile 删除文件或目录
func (s *fileService) DeleteFile(userID uint, path string) error {
	// 使用 rm -rf 强制删除，支持文件和目录
	// Escape path to prevent command injection somewhat, though sftp is safer.
	// Simple escaping for single quotes
	safePath := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	cmd := fmt.Sprintf("rm -rf %s", safePath)

	_, err := s.executeSSHCommand(userID, cmd)
	return err
}

// UnzipFile 解压文件
func (s *fileService) UnzipFile(userID uint, req *request.UnzipRequest) error {
	// unzip -o <path> -d <targetPath>
	// Check if unzip is installed? Assuming yes.

	safePath := "'" + strings.ReplaceAll(req.Path, "'", "'\\''") + "'"
	safeTarget := "'" + strings.ReplaceAll(req.TargetPath, "'", "'\\''") + "'"

	cmd := fmt.Sprintf("unzip -o %s -d %s", safePath, safeTarget)
	_, err := s.executeSSHCommand(userID, cmd)
	return err
}

// GetDiskUsage 计算目录大小
func (s *fileService) GetDiskUsage(userID uint, path string) (*dto.DiskUsageData, error) {
	// du -sb <path> | awk '{print $1}'
	safePath := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", safePath)

	output, err := s.executeSSHCommand(userID, cmd)
	if err != nil {
		return nil, err
	}

	sizeStr := strings.TrimSpace(output)
	size, _ := strconv.ParseInt(sizeStr, 10, 64)

	// Get filesystem usage percentage
	cmdUsage := fmt.Sprintf("df --output=pcent %s | tail -1 | tr -dc '0-9'", safePath)
	outputUsage, _ := s.executeSSHCommand(userID, cmdUsage)
	usage, _ := strconv.ParseFloat(strings.TrimSpace(outputUsage), 64)

	return &dto.DiskUsageData{
		DirectorySize: size,
		DiskUsage:     usage,
	}, nil
}

// Chmod 修改权限
func (s *fileService) Chmod(userID uint, req *request.ChmodRequest) error {
	// 使用 chmod 命令
	safePath := "'" + strings.ReplaceAll(req.Path, "'", "'\\''") + "'"
	safeMode := "'" + strings.ReplaceAll(req.Mode, "'", "'\\''") + "'"

	cmd := fmt.Sprintf("chmod %s %s", safeMode, safePath)
	_, err := s.executeSSHCommand(userID, cmd)
	return err
}

// CalculateSize 计算目录或文件大小
func (s *fileService) CalculateSize(userID uint, path string) (int64, string, float64, error) {
	// 防止命令注入，对路径进行转义
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"

	// 1. 使用 du -sb 计算字节大小
	cmd := fmt.Sprintf("du -sb %s | awk '{print $1}'", quotedPath)
	out, err := s.executeSSHCommand(userID, cmd)

	var bytes int64

	if err != nil {
		// 如果 du -sb 失败，尝试 du -sk (千字节)
		cmd = fmt.Sprintf("du -sk %s | awk '{print $1}'", quotedPath)
		out, err = s.executeSSHCommand(userID, cmd)
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
	// df -B1 <path> | tail -1 | awk '{print $2}' (Total size in bytes)
	cmdTotal := fmt.Sprintf("df -B1 %s | tail -1 | awk '{print $2}'", quotedPath)
	outTotal, err := s.executeSSHCommand(userID, cmdTotal)

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
