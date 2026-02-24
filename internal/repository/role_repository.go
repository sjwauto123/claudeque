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

// Create 创建角色
func (r *roleRepository) Create(role *entity.Role) error {
	return r.db.Create(role).Error
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

// ListAll 获取所有角色（仅名称与标识）
func (r *roleRepository) ListAll() ([]entity.Role, error) {
	var roles []entity.Role
	if err := r.db.Model(&entity.Role{}).Select("id", "name", "slug", "status").Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
