package request

import "mime/multipart"

// FileListRequest 文件列表查询请求
type FileListRequest struct {
	Path     string `form:"path" json:"path"`           // 目录路径
	Keyword  string `form:"keyword" json:"keyword"`     // 模糊搜索关键词
	Page     int    `form:"page" json:"page"`           // 第几页
	PageSize int    `form:"page_size" json:"page_size"` // 每页条数
}

// DiskUsageRequest 计算目录磁盘占比和大小
type DiskUsageRequest struct {
	Path string `json:"path" binding:"required"` // 目录路径
}

// DeleteFileRequest 删除文件或目录请求
type DeleteFileRequest struct {
	Path string `json:"path" binding:"required"` // 要删除的文件或目录路径
}

// UploadFileRequest 上传文件请求
type UploadFileRequest struct {
	File       *multipart.FileHeader `form:"file" binding:"required"`        // 上传的文件
	TargetPath string                `form:"target_path" binding:"required"` // 目标目录路径
}

// DownloadRequest 下载文件请求
type DownloadRequest struct {
	Path string `json:"path" binding:"required"` // 要下载的文件路径
}

// UnzipRequest 解压文件请求
type UnzipRequest struct {
	Path       string `json:"path" binding:"required"`       // 要解压的文件路径
	Filename   string `json:"filename" binding:"required"`   // 解压后的文件名
	TargetPath string `json:"targetpath" binding:"required"` // 解压到的路径
}
