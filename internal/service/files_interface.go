package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
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
}
