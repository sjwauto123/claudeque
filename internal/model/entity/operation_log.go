package entity

import "time"

// OperationLog 用户操作日志实体
type OperationLog struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement;comment:日志ID" json:"id"`
	Username    string    `gorm:"type:varchar(50);not null;comment:操作用户名称，关联users.username;index:idx_user_operation" json:"username"`
	ActionType  string    `gorm:"type:varchar(50);not null;comment:操作类型 (Login, SubmitJob, CancelJob, DeleteFile...);index:idx_action_type" json:"action_type"`
	Description string    `gorm:"type:text;comment:操作详情描述" json:"description"`
	Status      int       `gorm:"type:int;default:1;comment:操作状态" json:"status"`
	CreatedAt   time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;comment:操作时间;index:idx_operation_time;index:idx_user_operation" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}
