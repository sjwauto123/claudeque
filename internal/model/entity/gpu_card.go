package entity

// GpuCard GPU显卡实体
type GpuCard struct {
	ID           int    `json:"id" gorm:"primaryKey" json:"id"`
	UUID         string `json:"uuid" gorm:"column:uuid;type:varchar(100);uniqueIndex"`         // GPU 唯一标识符
	Name         string `json:"name" gorm:"column:name;type:varchar(50);not null;uniqueIndex"` // 显卡名称，如 gpu-0, gpu-1
	Index        int    `json:"index" gorm:"column:index;type:int;not null"`                   // 显卡在系统中的当前索引
	Status       int    `json:"status" gorm:"column:status;type:tinyint;not null;default:0"`   // 状态：0-空闲，1-忙碌
	CurrentJobID *int   `json:"current_job_id" gorm:"column:current_job_id;index"`             // 当前执行的任务ID
	GpuType      string `json:"gpu_type" gorm:"column:gpu_type"`                               // 显卡型号
	Memory       int    `json:"memory" gorm:"column:memory"`                                   // 显卡内存大小
}

// Status constants
const (
	GpuStatusIdle = 0 // 空闲
	GpuStatusBusy = 1 // 忙碌
)

func (GpuCard) TableName() string {
	return "gpu_cards"
}
