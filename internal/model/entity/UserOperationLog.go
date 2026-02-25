package entity

import (
	"time"
)

type UserOperationLog struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Username    string    `gorm:"size:100;not null;index" json:"username"`
	Method      string    `gorm:"size:10;not null" json:"method"`
	RequestData string    `gorm:"type:text;column:request_data" json:"requestData"`
	ActionType  string    `gorm:"size:50;not null;index;column:action_type" json:"actionType"`
	Path        string    `gorm:"size:255;not null" json:"path"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 表名
func (UserOperationLog) TableName() string {
	return "operation_logs"
}
