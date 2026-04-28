package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"context"
	"io"
	"mime/multipart"
	"time"
)

// FileService 文件服务接口
type FileService interface {
	GetFileList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error)
	UploadFile(userID int, file multipart.File, header *multipart.FileHeader, targetPath string, isRootMode bool) (*dto.FileUploadData, error)
	DownloadFile(userID int, path string, isRootMode bool) (io.ReadCloser, string, int64, time.Time, error)
	DeleteFile(userID int, path string, isRootMode bool) error
	UnzipFile(userID int, req *request.UnzipRequest, isRootMode bool) error
	GetDiskUsage(userID int, path string, isRootMode bool) (*dto.DiskUsageData, error)
	CalculateSize(userID int, path string, isRootMode bool) (int64, string, float64, error)
	GetHomeDirectoriesList(userID int, req *request.FileListRequest, isRootMode bool) (*dto.FilesListData, error)
	GetUploadProgress(userID int, filename string, targetPath string) (*dto.UploadProgressData, error)
	// SetDiskUsageCache 设置磁盘使用缓存
	SetDiskUsageCache(ctx context.Context, userID int, path string, data *dto.DiskUsageData) error
	// GetDiskUsageCache 获取磁盘使用缓存
	GetDiskUsageCache(ctx context.Context, userID int, path string) (*dto.DiskUsageData, error)
	// CalculateAllUsersDiskUsage 计算所有用户磁盘使用情况
	CalculateAllUsersDiskUsage() error
	// GetAllUsersDiskUsage 获取所有用户磁盘使用情况列表
	GetAllUsersDiskUsage() ([]*dto.UserDiskUsageData, error)
}
