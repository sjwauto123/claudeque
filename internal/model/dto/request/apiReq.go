package request

import "cloudque/internal/model/entity"

// APIPageQueryRequest API分页查询请求
type APIPageQueryRequest struct {
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"pageSize" binding:"required,min=1,max=100"`
	HTTPPath string `form:"http_path" binding:"omitempty"`
	Status   *int   `form:"status" binding:"omitempty,oneof=0 1"`
}

// CreateAPIRequest 创建API请求
type CreateAPIRequest struct {
	entity.BaseEntity
	Name       string `json:"name" binding:"required,min=1,max=255"`
	Category   string `json:"category" binding:"required"`
	Type       string `json:"type" binding:"omitempty"`
	Slug       string `json:"slug" binding:"required,min=1,max=50"`
	Status     *int   `json:"status" binding:"oneof=0 1"` // 0=停用, 1=启用
	HTTPMethod string `json:"http_method" binding:"required"`
	HTTPPath   string `json:"http_path" binding:"omitempty,max=65535"`
	Sort       int    `json:"sort" binding:"omitempty"`
}

// UpdateAPIRequest 更新API请求
type UpdateAPIRequest struct {
	ID          int    `json:"id" binding:"required"`
	Name        string `json:"name" binding:"omitempty,min=1,max=50"`
	Description string `json:"description" binding:"omitempty"`
	Category    string `json:"category" binding:"omitempty"`
	Slug        string `json:"slug" binding:"omitempty,min=1,max=50"`
	Type        string `json:"type" binding:"omitempty"`             // 菜单类型 3种: catalogue menu permission
	Status      *int   `json:"status" binding:"omitempty,oneof=0 1"` // 0=停用, 1=启用
	HTTPMethod  string `json:"http_method" binding:"omitempty"`
	HTTPPath    string `json:"http_path" binding:"omitempty,max=65535"`
	Sort        *int   `json:"sort" binding:"omitempty"`
}

// BatchDeleteAPIRequest 批量删除API请求
type BatchDeleteAPIRequest struct {
	IDs []int `json:"ids" binding:"required,min=1"`
}
