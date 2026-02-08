package repository

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// QueueRepository 队列数据层接口
type QueueRepository interface {
	// Add 添加任务到队列
	Add(ctx context.Context, jobID int, score float64) error
	// Remove 从队列移除任务
	Remove(ctx context.Context, jobID int) error
	// Peek 获取队首任务 (jobID, score, error)
	Peek(ctx context.Context) (int, float64, error)
	// NextSeq 获取下一个序列号
	NextSeq(ctx context.Context) (int, error)
	// Range 获取队列指定范围的任务ID列表
	Range(ctx context.Context, start, stop int64) ([]string, error)
	// RangeWithScores 获取队列指定范围的任务(带分数)
	RangeWithScores(ctx context.Context, start, stop int64) ([]redis.Z, error)
	// Rank 获取任务排名 (从0开始)
	Rank(ctx context.Context, jobID int) (int64, error)
	// Score 获取任务分数
	Score(ctx context.Context, jobID int) (float64, error)
}
