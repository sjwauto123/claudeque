package repository

import (
	"cloudque/internal/model/entity"
	"errors"
	"gorm.io/gorm"
)

// roleRepository 角色仓储实现
type roleRepository struct {
	db *gorm.DB
}

// NewRoleRepository 创建角色仓储
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}
func (r *roleRepository) GetRoleByID(id int) (*entity.Role, error) {
	var role entity.Role
	err := r.db.First(&role, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// Create 创建角色

// FindBySlug 根据 Slug 查找角色，权限，菜单
func (r *roleRepository) FindBySlug(slug string) (*entity.Role, error) {
	var role entity.Role

	err := r.db.Preload("Permissions").Preload("Menus").Where("slug = ?", slug).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}
func (r *roleRepository) PageList(offset, limit int, name string, status *int) ([]*entity.Role, int64, error) {
	var roles []*entity.Role
	var total int64

	// 构建查询条件
	query := r.db.Model(&entity.Role{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 获取符合条件的总记录数
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err = query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&roles).Error
	if err != nil {
		return nil, 0, err
	}

	return roles, total, nil

}
func (r *roleRepository) Create(role *entity.Role) error {
	return r.db.Create(role).Error
}

func (r *roleRepository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&entity.Role{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *roleRepository) Delete(id int) error {
	return r.db.Delete(&entity.Role{}, id).Error
}
func (r *roleRepository) BatchDelete(ids []int) error {
	return r.db.Where("id IN ?", ids).Delete(&entity.Role{}).Error
}

//func (r *roleRepository) GetRolePermissionByID(roleID int) (
//	menus []entity.Menu,
//	permissions []entity.Permission,
//	permissionMenus []entity.PermissionMenu,
//	roleMenuIDs map[int]bool,
//	rolePermissionIDs map[int]bool,
//	err error,
//) {
//
//	// 1. 所有菜单
//	if err = r.db.
//		Where("status = ?", 1).
//		Order("sort ASC").
//		Find(&menus).Error; err != nil {
//		return
//	}
//
//	// 2. 所有权限
//	if err = r.db.
//		Where("status = ? AND type = ?", 1, "permission").
//		Order("sort ASC").
//		Find(&permissions).Error; err != nil {
//		return
//	}
//	// 3. 查询权限菜单关联
//	if err = r.db.
//		Find(&permissionMenus).Error; err != nil {
//		return
//	}
//
//	// 4. 角色菜单ID
//	var menuIDs []int
//	if err = r.db.
//		Model(&entity.RoleMenu{}).
//		Select("menu_id").
//		Where("role_id = ?", roleID).
//		Scan(&menuIDs).Error; err != nil {
//		return
//	}
//
//	roleMenuIDs = make(map[int]bool)
//	for _, id := range menuIDs {
//		roleMenuIDs[id] = true
//	}
//
//	// 5. 角色权限ID
//	var permIDs []int
//	if err = r.db.
//		Model(&entity.RolePermission{}).
//		Select("permission_id").
//		Where("role_id = ?", roleID).
//		Scan(&permIDs).Error; err != nil {
//		return
//	}
//
//	rolePermissionIDs = make(map[int]bool)
//	for _, id := range permIDs {
//		rolePermissionIDs[id] = true
//	}
//	return
//}

func (r *roleRepository) GetRolePermissionByID(roleID int) (
	menus []entity.Menu,
	perms []entity.Permission,
	roleMenuMap map[int]bool,
	rolePermMap map[int]bool,
	err error,
) {

	// 1. 菜单
	if err = r.db.
		Where("status = ?", 1).
		Order("sort asc").
		Find(&menus).Error; err != nil {
		return
	}

	// 2. API 权限
	if err = r.db.
		Where("status = ?", 1).
		Order("sort asc").
		Find(&perms).Error; err != nil {
		return
	}

	// 3. 角色菜单
	var menuIDs []int
	r.db.Model(&entity.RoleMenu{}).
		Select("menu_id").
		Where("role_id = ?", roleID).
		Scan(&menuIDs)

	roleMenuMap = make(map[int]bool)
	for _, id := range menuIDs {
		roleMenuMap[id] = true
	}

	// 4. 角色权限
	var permIDs []int
	r.db.Model(&entity.RolePermission{}).
		Select("permission_id").
		Where("role_id = ?", roleID).
		Scan(&permIDs)

	rolePermMap = make(map[int]bool)
	for _, id := range permIDs {
		rolePermMap[id] = true
	}
	return
}
func (r *roleRepository) UpdateRolePermission(
	roleID int,
	menuIDs []int,
	permissionIDs []int,
) error {

	return r.db.Transaction(func(tx *gorm.DB) error {

		// 1. 删除旧菜单
		if err := tx.
			Where("role_id = ?", roleID).
			Delete(&entity.RoleMenu{}).Error; err != nil {
			return err
		}

		// 2. 删除旧权限
		if err := tx.
			Where("role_id = ?", roleID).
			Delete(&entity.RolePermission{}).Error; err != nil {
			return err
		}

		// 3. 写入新菜单
		if len(menuIDs) > 0 {
			var roleMenus []entity.RoleMenu
			for _, menuID := range menuIDs {
				roleMenus = append(roleMenus, entity.RoleMenu{
					RoleID: roleID,
					MenuID: menuID,
				})
			}

			if err := tx.Create(&roleMenus).Error; err != nil {
				return err
			}
		}

		// 4. 写入新权限
		if len(permissionIDs) > 0 {
			var rolePermissions []entity.RolePermission
			for _, permID := range permissionIDs {
				rolePermissions = append(rolePermissions, entity.RolePermission{
					RoleID:       roleID,
					PermissionID: permID,
				})
			}

			if err := tx.Create(&rolePermissions).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *roleRepository) ExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Role{}).
		Where("name = ?", name).
		Count(&count).Error
	return count > 0, err
}

func (r *roleRepository) ExistsBySlug(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Role{}).
		Where("name = ?", slug).
		Count(&count).Error
	return count > 0, err
}
func (r *roleRepository) ExistsByNameExcludingID(slug string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Role{}).
		Where("name = ?", slug).
		Where("id != ?", excludeID). // 排除自身数据

		Count(&count).Error
	return count > 0, err
}

func (r *roleRepository) ExistsBySlugExcludingID(slug string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Role{}).
		Where("slug = ?", slug).
		Where("id != ?", excludeID). // 排除自身数据
		Count(&count).Error
	return count > 0, err
}

// ListAll 获取所有角色（仅名称与标识）
func (r *roleRepository) ListAll() ([]entity.Role, error) {
	var roles []entity.Role
	if err := r.db.Model(&entity.Role{}).Select("id", "name", "slug", "status").Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
func (r *roleRepository) CheckUserPermission(userID int, method string, path string) (bool, error) {
	var count int64
	err := r.db.Table("admin_users").
		Joins("JOIN admin_role_users ru ON ru.user_id = admin_users.id").
		Joins("JOIN admin_roles ro ON ro.id = ru.role_id").
		Joins("JOIN admin_role_permissions rp ON rp.role_id = ro.id").
		Joins("JOIN admin_permissions p ON p.id = rp.permission_id").
		Where("admin_users.id = ?", userID).
		Where("p.http_method = ?", method).
		Where("p.http_path = ?", path).
		Where("p.status = ?", 1).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
