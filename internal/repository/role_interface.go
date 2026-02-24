package repository

import (
	"cloudque/internal/model/entity"
)

// RoleRepository 角色仓储接口
type RoleRepository interface {
	// FindBySlug 根据 Slug 查找角色（包含权限）
	FindBySlug(slug string) (*entity.Role, error)
	// ListAll 获取所有角色（名称与标识）
	ListAll() ([]entity.Role, error)
}
