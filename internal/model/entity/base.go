package entity

import (
	"time"

	"gorm.io/gorm"
)

// BaseEntity 基础实体
type BaseEntity struct {
	ID        int            `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate GORM hook - 创建前
func (b *BaseEntity) BeforeCreate(tx *gorm.DB) error {
	return nil
}

// BeforeUpdate GORM hook - 更新前
func (b *BaseEntity) BeforeUpdate(tx *gorm.DB) error {
	return nil
}
