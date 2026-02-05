package response

import "time"

// FilesListData 文件列表数据
type FilesListData struct {
	Breadcrumb    []*BreadcrumbItem `json:"breadcrumb"`     // 面包屑导航
	Path          string            `json:"path"`           // 当前路径
	FileList      []*FileItem       `json:"file_list"`      // 文件列表
	DirectoryList []*DirectoryItem  `json:"directory_list"` // 目录列表
}

// FileItem 文件信息
type FileItem struct {
	Filename  string    `json:"filename"`
	FileSize  int64     `json:"file_size"`
	UpdatedAt time.Time `json:"updated_at"` // 或者 string，视 needs 而定，这里用 time.Time 方便格式化
	Path      string    `json:"path"`
}

// DirectoryItem 目录信息
type DirectoryItem struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	Path      string    `json:"path"`
}

// DiskUsageData 目录磁盘占比和大小
type DiskUsageData struct {
	DirectorySize int64   `json:"directory_size"`
	DiskUsage     float64 `json:"disk_usage"`
}

// FileUploadData 文件上传响应
type FileUploadData struct {
	Filename string `json:"filename"`
}

// BreadcrumbItem 面包屑单项
type BreadcrumbItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
