package service

import (
	"context"
)

type GpuService interface {
	InitializeGpus(ctx context.Context) error
	GetIdleCount(ctx context.Context) (int, error)
	AcquireCards(ctx context.Context, count int, jobID int) ([]int, error)
	ReleaseCards(ctx context.Context, cardIDs []int) error
}
