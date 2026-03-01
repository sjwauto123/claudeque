package request

import "cloudque/internal/model/entity"

type RolePageQueryRequest struct {
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"pageSize" binding:"required,min=1,max=100"`
	Name     string `form:"name" binding:"omitempty,max=50"`
	Status   *int   `form:"status" binding:"omitempty,oneof=0 1"`
}
type CreateRoleRequest struct {
	entity.BaseEntity
	Name   string `json:"name" binding:"required,min=2,max=50"`
	Status *int   `json:"status" binding:"required,oneof=0 1"` // 0=禁用, 1=启用
	Slug   string `json:"slug" binding:"required"`
}

type UpdateRoleRequest struct {
	ID     int    `json:"id"`
	Name   string `json:"name" binding:"omitempty,max=50"`
	Status *int   `json:"status" binding:"omitempty,oneof=0 1"`
	Slug   string `json:"slug" binding:"omitempty"`
}
type UpdateRolePermissionRequest struct {
	MenuIDs       []int `json:"menu_ids" binding:"required"`
	PermissionIDs []int `json:"permission_ids" binding:"required"`
}
