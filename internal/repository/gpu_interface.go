package repository

import (
	"cloudque/internal/model/entity"
	"context"
)

// GpuRepository GPU仓储接口
type GpuRepository interface {
	// FindByName 根据名称查找GPU
	FindByName(ctx context.Context, name string) (*entity.GpuCard, error)
	// Create 创建GPU
	Create(ctx context.Context, gpu *entity.GpuCard) error
	// GetIdleCount 获取空闲显卡数量
	GetIdleCount(ctx context.Context) (int64, error)
	// Acquire 占用显卡 (包含事务处理)
	Acquire(ctx context.Context, count int, jobID int) ([]entity.GpuCard, error)
	// Release 释放显卡
	Release(ctx context.Context, cardIDs []int) error
}

// GpuCacheRepository GPU缓存接口
type GpuCacheRepository interface {
	// SetBusy 设置GPU为忙碌状态
	SetBusy(ctx context.Context, gpuID int, jobID int) error
	// SetIdle 设置GPU为空闲状态
	SetIdle(ctx context.Context, gpuID int) error
}
