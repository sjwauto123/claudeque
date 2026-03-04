package repository

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type queueRepository struct {
	redis *redis.Client
}

// NewQueueRepository 创建队列仓储
func NewQueueRepository(redis *redis.Client) QueueRepository {
	return &queueRepository{redis: redis}
}

const (
	QueueKey = "queue:jobs"
	SeqKey   = "queue:seq"
)

func (r *queueRepository) Add(ctx context.Context, jobID int, score float64) error {
	return r.redis.ZAdd(ctx, QueueKey, redis.Z{
		Score:  score,
		Member: strconv.Itoa(jobID),
	}).Err()
}

func (r *queueRepository) Remove(ctx context.Context, jobID int) error {
	return r.redis.ZRem(ctx, QueueKey, strconv.Itoa(jobID)).Err()
}

func (r *queueRepository) Peek(ctx context.Context) (int, float64, error) {
	items, err := r.redis.ZRangeWithScores(ctx, QueueKey, 0, 0).Result()
	if err != nil {
		return 0, 0, err
	}
	if len(items) == 0 {
		return 0, 0, nil // 空队列
	}
	jobID, err := strconv.Atoi(items[0].Member.(string))
	if err != nil {
		return 0, 0, err
	}
	return jobID, items[0].Score, nil
}

func (r *queueRepository) NextSeq(ctx context.Context) (int, error) {
	val, err := r.redis.Incr(ctx, SeqKey).Result()
	return int(val), err
}

func (r *queueRepository) Range(ctx context.Context, start, stop int64) ([]string, error) {
	return r.redis.ZRange(ctx, QueueKey, start, stop).Result()
}

func (r *queueRepository) RangeWithScores(ctx context.Context, start, stop int64) ([]redis.Z, error) {
	return r.redis.ZRangeWithScores(ctx, QueueKey, start, stop).Result()
}

func (r *queueRepository) Rank(ctx context.Context, jobID int) (int64, error) {
	return r.redis.ZRank(ctx, QueueKey, strconv.Itoa(jobID)).Result()
}

func (r *queueRepository) Score(ctx context.Context, jobID int) (float64, error) {
	return r.redis.ZScore(ctx, QueueKey, strconv.Itoa(jobID)).Result()
}
