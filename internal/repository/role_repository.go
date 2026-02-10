package repository

import (
	"errors"

	"cloudque/internal/model/entity"

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
