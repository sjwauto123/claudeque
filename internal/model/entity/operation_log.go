package entity

import "time"

type OperationLog struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username    string    `gorm:"size:100;not null;index" json:"username"`
	Object      string    `gorm:"size:200" json:"object"`
	ActionType  string    `gorm:"size:50;not null;index" json:"action_type"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"size:20;not null;default:0" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (OperationLog) TableName() string {
	return "operation_logs"
}
