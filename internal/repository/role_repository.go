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
func (r *roleRepository) GetRolePermission(roleID int) (
	menus []entity.Menu,
	permissions []entity.Permission,
	permIDs []int,
	err error,
) {
	// 1. 查询菜单（catalogue/menu）
	if err := r.db.
		Where("status = ?", 1).
		Order("sort ASC").
		Find(&menus).Error; err != nil {
		return nil, nil, nil, err
	}

	// 2. 查询角色权限 ID
	if err := r.db.
		Model(&entity.RolePermission{}).
		Select("permission_id").
		Where("role_id = ?", roleID).
		Scan(&permIDs).Error; err != nil {
		return nil, nil, nil, err
	}

	// 3. 查询权限（permission）
	if err := r.db.
		Where("status = ? AND type = ?", 1, "permission").
		Order("sort ASC").
		Find(&permissions).Error; err != nil {
		return nil, nil, nil, err
	}

	return menus, permissions, permIDs, nil
}

func (r *roleRepository) UpdateRolePermission(roleID int, permIDs []int) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		} else if err := tx.Error; err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// 1. 删除旧权限
	err := tx.Where("role_id = ?", roleID).Delete(&entity.RolePermission{}).Error
	if err != nil {
		return err
	}

	// 2. 插入新权限（仅当 permIDs 非空）
	if len(permIDs) > 0 {
		var rolePerms []entity.RolePermission
		for _, pid := range permIDs {
			rolePerms = append(rolePerms, entity.RolePermission{
				RoleID:       roleID,
				PermissionID: pid,
			})
		}
		err := tx.Create(&rolePerms).Error
		if err != nil {
			return err
		}
	}
	return nil
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
