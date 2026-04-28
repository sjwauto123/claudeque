package response

import "time"

// FilesListData 文件列表数据
type FilesListData struct {
	Breadcrumb    []*BreadcrumbItem `json:"breadcrumb"`     // 面包屑导航
	Path          string            `json:"path"`           // 当前路径
	FileList      []*FileItem       `json:"file_list"`      // 文件列表
	DirectoryList []*DirectoryItem  `json:"directory_list"` // 目录列表
	Total         int               `json:"total"`          // 总数
	Page          int               `json:"page"`           // 当前页码
	Pages         int               `json:"pages"`          // 总页数
	PageSize      int               `json:"pageSize"`       // 每页数量
}

// FileItem 文件信息
type FileItem struct {
	Filename  string    `json:"filename"`
	FileSize  int64     `json:"file_size"`
	UpdatedAt time.Time `json:"updated_at"` // 或者 string，视 needs 而定，这里用 time.Time 方便格式化
	Path      string    `json:"path"`
	Username  string    `json:"username"`
	IsDir     bool      `json:"is_dir"`
}

// DirectoryItem 目录信息
type DirectoryItem struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	Path      string    `json:"path"`
	Username  string    `json:"username"`
}

// DiskUsageData 目录磁盘占比和大小
type DiskUsageData struct {
	DirectorySize int64   `json:"directory_size"`
	DiskUsage     float64 `json:"disk_usage"`
}

// UserDiskUsageData 用户磁盘使用数据
type UserDiskUsageData struct {
	UserID        int64   `json:"user_id"`
	Username      string  `json:"username"`
	DirectorySize int64   `json:"directory_size"`
	DiskUsage     float64 `json:"disk_usage"`
	HomeDirectory string  `json:"home_directory"`
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

// UploadProgressData 上传进度数据
type UploadProgressData struct {
	Filename  string  `json:"filename"`
	Status    string  `json:"status"`    // pending, uploading, completed, failed
	Progress  float64 `json:"progress"`  // 0-100
	Uploaded  int64   `json:"uploaded"`  // 已上传字节数
	Total     int64   `json:"total"`     // 总字节数
	Speed     string  `json:"speed"`     // 上传速度
	Remaining string  `json:"remaining"` // 剩余时间
	Error     string  `json:"error,omitempty"`
}
