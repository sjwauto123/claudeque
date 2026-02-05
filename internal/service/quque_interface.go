package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"context"
	"time"
)

type QueueService interface {
	Enqueue(ctx context.Context, jobID uint, priority int) error
	Peek(ctx context.Context) (*entity.Item, error)
	Remove(ctx context.Context, jobID uint) error
	GetQueuePage(ctx context.Context, req request.JobListRequest, startTime, endTime time.Time) ([]response.QueueJobResponse, int64, int, int, error)
	MoveBefore(ctx context.Context, jobID uint, beforeJobID uint) error
	GetFrontCount(ctx context.Context, jobID uint) (int, error)
}
