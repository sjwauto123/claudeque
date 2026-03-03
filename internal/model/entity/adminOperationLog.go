package entity

import "time"

type AdminOperationLog struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Username   string    `gorm:"size:100;not null" json:"username"` // 添加用户名字段，便于查询
	ActionType string    `gorm:"size:50;not null;index" json:"action_type"`
	Status     int       `gorm:"default:1" json:"status"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AdminOperationLog) TableName() string {
	return "admin_operation_log"
}
