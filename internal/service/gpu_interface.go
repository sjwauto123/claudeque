package service

import (
	"context"
)

type GpuService interface {
	// InitializeGpus 初始化GPU卡片
	InitializeGpus(ctx context.Context) error
	// GetIdleCount 获取空闲显卡数量
	GetIdleCount(ctx context.Context) (int, error)
	// AcquireCards 占用指定数量的显卡
	AcquireCards(ctx context.Context, count int, jobID int) ([]int, error)
	// ReleaseCards 释放指定显卡
	ReleaseCards(ctx context.Context, cardIDs []int) error
}
