package repository

import (
	"errors"

	"cloudque/internal/model/entity"

	"gorm.io/gorm"
)

// userRepository 用户仓储实现
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// FindByID 根据 ID 查找用户
func (r *userRepository) FindByID(id int) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Roles").First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername 根据用户名查找用户
func (r *userRepository) FindByUsername(username string) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Roles").Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查找用户
func (r *userRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Roles").Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Create 创建用户
func (r *userRepository) Create(user *entity.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *userRepository) Update(user *entity.User) error {

	return r.db.Model(user).Select(
		"username", "password", "avatar", "email",
		"status", "priority", "multi_training", "cross_server", "updated_at",
	).Updates(user).Error
}

// Delete 删除用户
func (r *userRepository) Delete(id int) error {
	return r.db.Delete(&entity.User{}, id).Error
}

// List 分页获取用户列表
func (r *userRepository) List(offset, limit int, username, email string, status *int) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64

	query := r.db.Model(&entity.User{})

	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := query.Preload("Roles").Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// AssignRoleByName 为用户分配指定角色（按名称）
func (r *userRepository) AssignRoleByName(userID int, name string) error {
	var role entity.Role
	if err := r.db.Where("slug = ?", name).First(&role).Error; err != nil {
		return err
	}
	user := entity.User{BaseEntity: entity.BaseEntity{ID: userID}}
	return r.db.Model(&user).Association("Roles").Append(&role)
}

// ClearRoles 清空用户的所有角色关联
func (r *userRepository) ClearRoles(userID int) error {
	user := entity.User{BaseEntity: entity.BaseEntity{ID: userID}}
	return r.db.Model(&user).Association("Roles").Clear()
}

// ReplaceRolesByNames 更改用户角色
func (r *userRepository) ReplaceRolesByNames(userID int, names []string) error {
	user := entity.User{BaseEntity: entity.BaseEntity{ID: userID}}
	if len(names) == 0 {
		return r.db.Model(&user).Association("Roles").Clear()
	}
	var roles []entity.Role
	if err := r.db.Where("slug IN ?", names).Find(&roles).Error; err != nil {
		return err
	}
	return r.db.Model(&user).Association("Roles").Replace(&roles)
}
