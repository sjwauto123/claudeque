package repository

import (
	"cloudque/internal/model/entity"
)

type RoleRepository interface {
	// GetRoleByID 根据 ID 获取用户信息
	GetRoleByID(id int) (*entity.Role, error)
	// PageList 分页查询用户列表
	PageList(offset, limit int, name string, status *int) ([]*entity.Role, int64, error)
	// Create 新建用户
	Create(role *entity.Role) error
	// Update 修改用户信息
	Update(role *entity.Role) error
	// Delete 删除用户
	Delete(id int) error
	// BatchDelete 批量删除用户
	BatchDelete(ids []int) error
	// GetRolePermissionByID 获取角色权限树
	GetRolePermissionByID(roleID int) (
		menus []entity.Menu,
		permissions []entity.Permission,
		permissionMenus []entity.PermissionMenu,
		roleMenuIDs map[int]bool,
		rolePermissionIDs map[int]bool,
		err error,
	)
	// UpdateRolePermission 更新角色权限
	UpdateRolePermission(roleID int, menuIDs []int, permissionIDs []int) error

	// ExistsByName 判断角色名是否存在
	ExistsByName(name string) (bool, error)
	// ExistsBySlug 判断角色标识是否存在
	ExistsBySlug(slug string) (bool, error)
}
