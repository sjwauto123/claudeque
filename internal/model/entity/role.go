package entity

import "time"

type Role struct {
	BaseEntity
	Name   string `gorm:"type:varchar(50);not null;comment:角色名称" json:"name"`
	Status int    `gorm:"type:int;default:1;comment:0-禁用 1-启用" json:"status"`
	Slug   string `gorm:"type:varchar(50);uniqueIndex;not null;comment:角色唯一标识" json:"slug"`

	Permissions []Permission `gorm:"many2many:admin_role_permissions;joinForeignKey:role_id;JoinReferences:permission_id" json:"permissions"`
}

func (Role) TableName() string {
	return "admin_roles"
}

// UserRole 用户角色关联表
type UserRole struct {
	RoleID    uint      `gorm:"primaryKey;comment:角色ID" json:"role_id"`
	UserID    uint      `gorm:"primaryKey;comment:用户ID" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserRole) TableName() string {
	return "admin_role_users"
}
