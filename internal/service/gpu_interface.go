package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"context"
)

type GpuService interface {
	// InitializeGpus 初始化GPU卡片
	InitializeGpus(ctx context.Context) error
	// CheckAvailable 检查显卡是否可用
	CheckAvailable(ctx context.Context, cardIDs []int) (bool, error)
	// AcquireCards 占用指定的显卡
	AcquireCards(ctx context.Context, cardIDs []int, jobID int) error
	// ReleaseCards 释放指定显卡
	ReleaseCards(ctx context.Context, cardIDs []int) error
	// SyncGpuCards 同步GPU卡片
	SyncGpuCards(ctx context.Context) error
	// GetGpuCardsByIDs 根据ID列表获取显卡信息
	GetGpuCardsByIDs(ctx context.Context, ids []int) ([]entity.GpuCard, error)
	// GetGpus 获取显卡信息
	GetGpus(ctx context.Context) (response.Gpus, error)
}
