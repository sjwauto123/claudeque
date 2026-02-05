package entity

import "time"

type Role struct {
	BaseEntity
	Name   string `gorm:"size:50;unique;not null" json:"name"`
	Status int    `gorm:"default:0" json:"status"`
	Slug   string `gorm:"size:50;unique;not null" json:"slug"`
}

func (Role) TableName() string {
	return "admin_roles"
}

type RolePermission struct {
	RoleID       int       `gorm:"primaryKey;column:role_id" json:"role_id"`
	PermissionID int       `gorm:"primaryKey;column:permission_id" json:"permission_id"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (RolePermission) TableName() string {
	return "admin_role_permissions"
}

type RoleMenu struct {
	RoleID    int       `gorm:"primaryKey;column:role_id" json:"role_id"`
	MenuID    int       `gorm:"primaryKey;column:menu_id" json:"menu_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (RoleMenu) TableName() string {
	return "admin_role_menu"
}
