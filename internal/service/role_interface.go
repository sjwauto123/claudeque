package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
)

type RoleService interface {
	// GetRoleByID 根据 ID 获取用户信息
	GetRoleByID(id int) (*entity.Role, error)
	// PageList 分页查询用户列表
	PageList(page *request.RolePageQueryRequest) ([]*entity.Role, int64, error)
	// Create 新建用户
	Create(req *request.CreateRoleRequest) error
	// Update 修改用户信息
	Update(req *request.UpdateRoleRequest) error
	// Delete 删除用户
	Delete(id int) error
	// BatchDelete 批量删除用户
	BatchDelete(ids []int) error
	// GetRolePermissionByID 获取角色权限树
	GetRolePermissionByID(roleID int) (*response.RolePermissionTree, error)
	// UpdateRolePermission 更新角色权限
	UpdateRolePermission(roleID int, permIDs []int) error
}
