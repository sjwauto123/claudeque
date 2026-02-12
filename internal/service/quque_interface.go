package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"context"
	"time"
)

type QueueService interface {
	// Enqueue 入队
	Enqueue(ctx context.Context, jobID int, priority int) error
	// Peek scheduler的方法
	Peek(ctx context.Context) (*entity.Item, error)
	// Remove 出队
	Remove(ctx context.Context, jobID int) error
	// GetQueuePage 分页查询，按条件检索，返回完整排队任务信息
	GetQueuePage(ctx context.Context, req request.QueueListRequest, startTime, endTime time.Time) ([]response.QueueJobResponse, int64, int, int, error)
	// MoveBefore 更新排队
	MoveBefore(ctx context.Context, jobID int, beforeJobID int) error
	// GetFrontCount 返回某个任务前方排队数量
	GetFrontCount(ctx context.Context, jobID int) (int, error)
}
