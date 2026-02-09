package entity

import (
	"time"
)

type OperationLog struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Username    string    `gorm:"size:100;not null;index" json:"username"` // 添加用户名字段，便于查询
	Object      string    `gorm:"size:200" json:"object"`
	ActionType  string    `gorm:"size:50;not null;index" json:"action_type"`
	Description string    `gorm:"type:text" json:"description"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 表名
func (OperationLog) TableName() string {
	return "operation_logs"
}
