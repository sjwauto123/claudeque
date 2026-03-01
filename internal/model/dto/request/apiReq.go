package request

// APIPageQueryRequest API分页查询请求
type APIPageQueryRequest struct {
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"pageSize" binding:"required,min=1,max=100"`
	HTTPPath string `form:"http_path" binding:"omitempty"`
	Status   *int   `form:"status" binding:"omitempty,oneof=0 1"`
}

// CreateAPIRequest 创建API请求
type CreateAPIRequest struct {
	ID         int    `json:"id" binding:"omitempty"`
	Name       string `json:"name" binding:"required,min=1,max=50"`
	Category   string `json:"category" binding:"required"`
	Slug       string `json:"slug" binding:"required,min=1,max=50"`
	Status     *int   `json:"status" binding:"required,oneof=0 1"` // 0=停用, 1=启用
	HTTPMethod string `json:"http_method" binding:"required,oneof=GET POST PUT DELETE"`
	HTTPPath   string `json:"http_path" binding:"required,max=65535"`
	Sort       int    `json:"sort" binding:"required"`
}

// UpdateAPIRequest 更新API请求
type UpdateAPIRequest struct {
	ID         int    `json:"id" binding:"required"`
	Name       string `json:"name" binding:"omitempty,min=1,max=50"`
	Category   string `json:"category" binding:"omitempty"`
	Slug       string `json:"slug" binding:"omitempty,min=1,max=50"`
	Type       string `json:"type" binding:"omitempty"`
	Status     *int   `json:"status" binding:"omitempty,oneof=0 1"`
	HTTPMethod string `json:"http_method" binding:"omitempty,oneof=GET POST PUT DELETE"`
	HTTPPath   string `json:"http_path" binding:"omitempty,max=65535"`
	Sort       *int   `json:"sort" binding:"omitempty"`
}

// BatchDeleteRequest 批量删除API请求
type BatchDeleteRequest struct {
	IDs []int `json:"ids" binding:"required"`
}
