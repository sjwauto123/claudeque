package entity

import "time"

type Job struct {
	ID          uint   `json:"id" gorm:"primaryKey" `
	Name        string `json:"name" gorm:"column:name;type:varchar(100);not null"`
	Description string `json:"description" gorm:"column:description;type:text"`
	UserId      uint   `json:"user_id" gorm:"column:user_id;index;not null"`
	FilePath    string `json:"file_path" gorm:"column:file_path;index"`                  // 训练脚本文件
	GpuCount    int    `json:"gpu_count" gorm:"column:gpu_count;type:tinyint;default:1"` // 需要的显卡数量
	//Priority   int        `json:"priority" gorm:"column:priority;type:tinyint;default:2"`   // 优先级：1-高，2-低
	Status    int        `json:"status" gorm:"column:status;index"`
	CreatedAt *time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	//StartedAt  *time.Time `json:"started_at" gorm:"column:started_at"`
	FinishedAt *time.Time `json:"finished_at" gorm:"column:finished_at"`

	LogPath string `json:"log_path" gorm:"column:log_path;type:varchar(255)"` // 日志文件路径
}

// JobStatus 任务状态常量
const (
	JobStatusPending    = 0 // 待执行
	JobStatusQueued     = 1 // 排队中
	JobStatusRunning    = 2 // 执行中
	JobStatusCompleted  = 3 // 已完成
	JobStatusFailed     = 4 // 失败
	JobStatusCancelled  = 5 // 被终止
	JobStatusWaitingGpu = 6 // 等待足够显卡
)

// JobPriority 优先级常量
const (
	JobPriorityHigh = 1 // 高优先级
	JobPriorityLow  = 2 // 低优先级
)

func (Job) TableName() string {
	return "jobs"
}
