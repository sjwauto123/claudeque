package entity

import (
	"time"
)

type Permission struct {
	BaseEntity
	Name       string `gorm:"type:varchar(50);not null;comment:权限名称" json:"name"`
	Category   string `gorm:"type:varchar(50);not null;comment:API类别" json:"category"`
	Slug       string `gorm:"type:varchar(50);uniqueIndex;not null;comment:权限唯一标识" json:"slug"`
	Type       string `gorm:"type:varchar(20);default:'';comment:权限类型" json:"type"`
	Status     int    `gorm:"type:int;default:1;comment:0-禁用 1-启用" json:"status"`
	HttpMethod string `gorm:"type:varchar(10);comment:API请求方法" json:"http_method"`
	HttpPath   string `gorm:"type:varchar(255);comment:API路径" json:"http_path"`
	Sort       int    `gorm:"type:int;default:1;comment:菜单排序" json:"sort"`
}

func (Permission) TableName() string {
	return "admin_permissions"
}

// RolePermission 角色权限关联表
type RolePermission struct {
	RoleID       int       `gorm:"primaryKey;comment:角色ID" json:"role_id"`
	PermissionID int       `gorm:"primaryKey;comment:权限ID" json:"permission_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (RolePermission) TableName() string {
	return "admin_role_permissions"
}
