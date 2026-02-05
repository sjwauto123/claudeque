package entity

import "time"

type Role struct {
	ID        uint      `gorm:"primaryKey;autoIncrement;comment:角色ID，主键" json:"id"`
	Name      string    `gorm:"type:varchar(50);not null;unique;comment:角色名称" json:"name"`
	Status    *int8     `gorm:"type:tinyint;comment:角色状态 0-禁用 1-启用" json:"status"`
	Slug      string    `gorm:"type:varchar(50);not null;unique;comment:角色唯一标识，代码中使用" json:"slug"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (Role) TableName() string {
	return "admin_roles"
}
