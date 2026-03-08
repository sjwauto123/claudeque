package repository

import (
	"cloudque/internal/model/entity"
	"context"
)

type ProcessRepository interface {
	// Create 创建进程信息
	Create(p *entity.Process) error
	// FindActiveByJobID 根据任务ID查找活跃进程记录
	FindActiveByJobID(jobID int) ([]entity.Process, error)
	// Update 更新进程记录
	Update(p *entity.Process) error
	// FindAll 获取所有活跃进程
	FindAll() ([]entity.Process, error)
}

// ProcessCacheRepository GPU缓存接口
type ProcessCacheRepository interface {
	// CreatePid 创建PID
	CreatePid(ctx context.Context, PID int, jobName string) error
	// DelPid 删除PID
	DelPid(ctx context.Context, jobName string) error
	// GetAllPid 获取全部PID
	GetAllPid(ctx context.Context) ([]string, []int, error)
}
