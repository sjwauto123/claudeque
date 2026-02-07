package entity

import "time"

type Permission struct {
	BaseEntity
	Name       string `gorm:"column:name;size:255;not null" json:"name"`
	Category   string `gorm:"column:category;size:255" json:"category"`
	Slug       string `gorm:"column:slug;size:50;not null" json:"slug"`
	Type       string `gorm:"column:type;size:50" json:"type"` // "catalogue", "menu", "permission"
	Sort       int    `gorm:"column:sort;size:11" json:"sort"`
	Status     int    `gorm:"column:status;size:4;not null" json:"status"`    // 0=禁用, 1=启用
	HTTPMethod string `gorm:"column:http_method;size:191" json:"http_method"` // e.g., "GET", "POST"
	HTTPPath   string `gorm:"column:http_path" json:"http_path"`              // 支持通配符，如 "/api/v1/users/*"
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
