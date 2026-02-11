package entity

import "time"

type Permission struct {
	BaseEntity
	Name       string `gorm:"not null;comment:权限名称" json:"name"`
	Category   string `gorm:"not null;comment:权限类别" json:"category"`
	Slug       string `gorm:"uniqueIndex;not null;comment:权限唯一标识" json:"slug"`
	Type       string `gorm:"comment:权限类型" json:"type"`
	Status     int    `gorm:"default:1;comment:0-禁用 1-启用" json:"status"`
	HTTPMethod string `gorm:"column:http_method;size:191" json:"http_method"`
	HTTPPath   string `gorm:"column:http_path" json:"http_path"`
	Sort       int    `gorm:"default:1;comment:菜单排序" json:"sort"`
}

func (Permission) TableName() string {
	return "admin_permissions"
}

type PermissionMenu struct {
	PermissionID int       `gorm:"primaryKey;column:permission_id" json:"permission_id"`
	MenuID       int       `gorm:"primaryKey;column:menu_id" json:"menu_id"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (PermissionMenu) TableName() string {
	return "admin_permission_menu"
}
