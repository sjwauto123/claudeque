package repository

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"context"
)

// GpuRepository GPU仓储接口
type GpuRepository interface {
	// FindByUUID 根据UUID查找GPU
	FindByUUID(ctx context.Context, uuid string) (*entity.GpuCard, error)
	// GetByIDs 根据ID列表获取GPU列表
	GetByIDs(ctx context.Context, ids []int) ([]entity.GpuCard, error)
	// Acquire 占用显卡 (包含事务处理)
	Acquire(ctx context.Context, cardIDs []int, jobID int) error
	// Release 释放显卡
	Release(ctx context.Context, cardIDs []int) error
	// SyncCards 同步显卡信息
	SyncCards(ctx context.Context, cards []entity.GpuCard) error
	// CheckAvailable 检查显卡是否可用
	CheckAvailable(ctx context.Context, cardIDs []int) (bool, error)
	// GetGpus 获取显卡信息
	GetGpus(ctx context.Context) ([]response.GpuSpec, int, error)
}

// GpuCacheRepository GPU缓存接口
type GpuCacheRepository interface {
	// SetBusy 设置GPU为忙碌状态
	SetBusy(ctx context.Context, gpuID int, jobID int) error
	// SetIdle 设置GPU为空闲状态
	SetIdle(ctx context.Context, gpuID int) error
	// CheckAvailable 检查显卡是否可用 (从缓存读取)
	CheckAvailable(ctx context.Context, cardIDs []int) (bool, error)
}
