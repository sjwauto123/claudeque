package repository

import (
	"cloudque/internal/model/entity"
	"errors"
	"gorm.io/gorm"
	"time"
)

type roleRepository struct {
	db *gorm.DB
}

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

func (r *roleRepository) Update(role *entity.Role) error {
	// 避免更新created_at引起mysql报错, 所以只修改需要更新的字段
	updates := make(map[string]interface{})
	if role.Name != "" {
		updates["name"] = role.Name
	}
	if role.Slug != "" {
		updates["slug"] = role.Slug
	}
	if role.Status != 0 { // 假设 0 是无效状态
		updates["status"] = role.Status
	}
	updates["updated_at"] = time.Now()

	return r.db.Model(&entity.Role{}).
		Where("id = ? AND deleted_at IS NULL", role.ID).
		Updates(updates).Error
}

func (r *roleRepository) Delete(id int) error {
	return r.db.Delete(&entity.Role{}, id).Error
}
func (r *roleRepository) BatchDelete(ids []int) error {
	return r.db.Where("id IN ?", ids).Delete(&entity.Role{}).Error
}
func (r *roleRepository) GetRolePermissionByID(roleID int) (
	menus []entity.Menu,
	permissions []entity.Permission,
	permissionMenus []entity.PermissionMenu,
	roleMenuIDs map[int]bool,
	rolePermissionIDs map[int]bool,
	err error,
) {

	// 1. 所有菜单
	if err = r.db.
		Where("status = ?", 1).
		Order("sort ASC").
		Find(&menus).Error; err != nil {
		return
	}

	// 2. 所有权限
	if err = r.db.
		Where("status = ? AND type = ?", 1, "permission").
		Order("sort ASC").
		Find(&permissions).Error; err != nil {
		return
	}
	// 3. 查询权限菜单关联
	if err = r.db.
		Find(&permissionMenus).Error; err != nil {
		return
	}

	// 4. 角色菜单ID
	var menuIDs []int
	if err = r.db.
		Model(&entity.RoleMenu{}).
		Select("menu_id").
		Where("role_id = ?", roleID).
		Scan(&menuIDs).Error; err != nil {
		return
	}

	roleMenuIDs = make(map[int]bool)
	for _, id := range menuIDs {
		roleMenuIDs[id] = true
	}

	// 5. 角色权限ID
	var permIDs []int
	if err = r.db.
		Model(&entity.RolePermission{}).
		Select("permission_id").
		Where("role_id = ?", roleID).
		Scan(&permIDs).Error; err != nil {
		return
	}

	rolePermissionIDs = make(map[int]bool)
	for _, id := range permIDs {
		rolePermissionIDs[id] = true
	}
	return
}

func (r *roleRepository) UpdateRolePermission(
	roleID int,
	menuIDs []int,
	permissionIDs []int,
) error {

	return r.db.Transaction(func(tx *gorm.DB) error {

		// 1. 除旧菜单
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

		// 3. 插入新菜单
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

		// 4. 插入新权限
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

// ExistsByName 判断角色名是否存在
func (r *roleRepository) ExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Role{}).
		Where("name = ?", name).
		Count(&count).Error
	return count > 0, err
}

// ExistsBySlug 判断角色标识是否存在
func (r *roleRepository) ExistsBySlug(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Role{}).
		Where("slug = ?", slug).
		Count(&count).Error
	return count > 0, err
}
