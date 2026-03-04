package entity

import "time"

type Job struct {
	ID          int        `json:"id" gorm:"primaryKey" `
	Name        string     `json:"name" gorm:"column:name;type:varchar(100);not null"`
	Description string     `json:"description" gorm:"column:description;type:text"`
	UserId      int        `json:"user_id" gorm:"column:user_id;index;not null"`
	FilePath    string     `json:"file_path" gorm:"column:file_path;index"` // 训练脚本文件
	GpuIDs      string     `json:"gpu_ids" gorm:"column:gpu_ids;type:varchar(255)"`
	Status      int        `json:"status" gorm:"column:status;index"`
	CreatedAt   *time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	StartedAt   *time.Time `json:"started_at" gorm:"column:started_at"`
	FinishedAt  *time.Time `json:"finished_at" gorm:"column:finished_at"`
}

// JobStatus 任务状态常量
const (
	JobStatusPending    = 0 // 待执行
	JobStatusQueued     = 1 // 排队中
	JobStatusRunning    = 2 // 执行中
	JobStatusCompleted  = 3 // 已完成
	JobStatusFailed     = 4 // 失败
	JobStatusCancelled  = 5 // 被终止
	JobStatusWaitingGpu = 6 // 等待显卡
)

// JobPriority 优先级常量
const (
	JobPriorityHigh = 2 // 高优先级
	JobPriorityLow  = 1 // 低优先级
)

func (Job) TableName() string {
	return "jobs"
}
