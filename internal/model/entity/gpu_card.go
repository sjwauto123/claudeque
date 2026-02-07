package entity

// GpuCard GPU显卡实体
type GpuCard struct {
	ID           int    `json:"id" gorm:"primaryKey" json:"id"`
	Name         string `json:"name" gorm:"column:name;type:varchar(50);not null;uniqueIndex"` // 显卡名称，如 gpu-0, gpu-1
	Status       int    `json:"status" gorm:"column:status;type:tinyint;not null;default:0"`   // 状态：0-空闲，1-忙碌
	CurrentJobID *int   `json:"current_job_id" gorm:"column:current_job_id;index"`             // 当前执行的任务ID
}

// Status constants
const (
	GpuStatusIdle = 0 // 空闲
	GpuStatusBusy = 1 // 忙碌
)

func (GpuCard) TableName() string {
	return "gpu_cards"
}
