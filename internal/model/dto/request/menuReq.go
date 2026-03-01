package request

import "cloudque/internal/model/entity"

// MenuPageQueryRequest 菜单分页查询请求
type MenuPageQueryRequest struct {
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"pageSize" binding:"required,min=1,max=100"`
	Title    string `form:"title" binding:"omitempty,max=50"`
	Status   *int   `form:"status" binding:"omitempty,oneof=0 1"`
}

// CreateMenuRequest 创建菜单请求
type CreateMenuRequest struct {
	entity.BaseEntity
	ParentID *int   `json:"parent_id" binding:"omitempty"`
	Title    string `json:"title" binding:"required,min=1,max=50"`
	Type     string `json:"type" binding:"required,oneof=catalogue menu"`
	Status   *int   `json:"status" binding:"required,oneof=0 1"`
	Icon     string `json:"icon" binding:"omitempty,max=50"`
	URI      string `json:"uri" binding:"required,min=1,max=50"`
	Sort     int    `json:"sort" binding:"required"`
}

// UpdateMenuRequest 更新菜单请求
type UpdateMenuRequest struct {
	ID       int    `json:"id" binding:"required"`
	Title    string `json:"title" binding:"omitempty,min=1,max=50"`
	Type     string `json:"type" binding:"omitempty,oneof=catalogue menu"`
	Status   *int   `json:"status" binding:"omitempty,oneof=0 1"`
	Icon     string `json:"icon" binding:"omitempty,max=50"`
	URI      string `json:"uri" binding:"omitempty,min=1,max=50"`
	Sort     *int   `json:"sort" binding:"omitempty"`
	ParentID int    `json:"parent_id" binding:"omitempty"`
}

// BatchDeleteMenuRequest 批量删除菜单请求
type BatchDeleteMenuRequest struct {
	IDs []int `json:"ids" binding:"required,min=1"`
}
