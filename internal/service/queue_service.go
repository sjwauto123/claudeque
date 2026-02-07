package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"

	"github.com/redis/go-redis/v9"
)

const (
	QueueKey = "queue:jobs"

	// PriorityGap 优先级基数
	PriorityGap = int(1_000_000_000)
)

type queueService struct {
	redis   *redis.Client
	jobRepo repository.JobRepository
}

func NewQueueService(redis *redis.Client, jobRepo repository.JobRepository) QueueService {
	return &queueService{redis: redis, jobRepo: jobRepo}
}

// 内部：获取全局自增序号
func (s *queueService) nextSeq(ctx context.Context) (int, error) {
	val, err := s.redis.Incr(ctx, "queue:seq").Result()
	return int(val), err
}

// Enqueue 入队
func (s *queueService) Enqueue(ctx context.Context, jobID int, priority int) error {
	seq, err := s.nextSeq(ctx)
	if err != nil {
		return err
	}

	base := int(priority) * PriorityGap

	score := base + seq

	return s.redis.ZAdd(ctx, QueueKey, redis.Z{
		Score:  float64(score),
		Member: strconv.Itoa(jobID),
	}).Err()
}

// Peek scheduler的方法
func (s *queueService) Peek(ctx context.Context) (*entity.Item, error) {
	items, err := s.redis.ZRangeWithScores(ctx, QueueKey, 0, 0).Result()
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, nil
	}

	jobID, err := strconv.Atoi(items[0].Member.(string))
	if err != nil {
		return nil, fmt.Errorf("解析任务ID失败: %w", err)
	}

	return &entity.Item{
		JobID: int(jobID),
		Score: int(items[0].Score),
	}, nil
}

// Remove 出队
func (s *queueService) Remove(ctx context.Context, jobID int) error {
	return s.redis.ZRem(ctx, QueueKey, strconv.Itoa(jobID)).Err()
}

// GetQueuePage 分页查询，按条件检索，返回完整排队任务信息
func (s *queueService) GetQueuePage(ctx context.Context, req request.JobListRequest, startTime, endTime time.Time) ([]response.QueueJobResponse, int, int, int, error) {
	members, err := s.redis.ZRange(ctx, QueueKey, 0, -1).Result()
	if err != nil {
		return nil, 0, 0, 0, err
	}

	if len(members) == 0 {
		return []response.QueueJobResponse{}, 0, req.Page, req.PageSize, err
	}

	orderedJobIDs := make([]int, 0, len(members))
	for _, m := range members {
		jobID, err := strconv.Atoi(m)
		if err != nil {
			continue
		}
		orderedJobIDs = append(orderedJobIDs, int(jobID))
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	rows, total, err := s.jobRepo.GetQueueJobListFiltered(orderedJobIDs, req, startTime, endTime)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	now := time.Now()
	list := make([]response.QueueJobResponse, 0, len(rows))
	for _, row := range rows {
		rank, err := s.redis.ZRank(ctx, QueueKey, strconv.Itoa(int(row.JobID))).Result()
		frontCount := 0
		if err == nil {
			frontCount = int(rank)
		}
		waitSec := int(now.Sub(row.SubmittedAt).Seconds())
		if waitSec < 0 {
			waitSec = 0
		}
		list = append(list, response.QueueJobResponse{
			JobID:       row.JobID,
			UserName:    row.UserName,
			JobName:     row.JobName,
			Description: row.Description,
			Status:      row.Status,
			SubmittedAt: row.SubmittedAt,
			WaitSeconds: waitSec,
			FrontCount:  frontCount,
		})
	}

	return list, total, req.Page, req.PageSize, nil
}
func (s *queueService) MoveBefore(ctx context.Context, jobID int, beforeJobID int) error {
	jobKey := strconv.Itoa(jobID)
	beforeKey := strconv.Itoa(beforeJobID)

	// 获取 beforeJob 的 rank
	rank, err := s.redis.ZRank(ctx, QueueKey, beforeKey).Result()
	if err != nil {
		return err
	}

	// 获取 beforeJob 的 score
	beforeScore, err := s.redis.ZScore(ctx, QueueKey, beforeKey).Result()
	if err != nil {
		return err
	}

	var newScore float64
	if rank == 0 {
		// 插到最前面
		newScore = beforeScore - float64(PriorityGap)
	} else {
		// 拿前一个元素
		prev, err := s.redis.ZRangeWithScores(ctx, QueueKey, rank-1, rank-1).Result()
		if err != nil {
			return err
		}
		prevScore := prev[0].Score
		// 取中点
		newScore = (prevScore + beforeScore) / 2
	}

	// 更新 score
	return s.redis.ZAdd(ctx, QueueKey, redis.Z{
		Score:  newScore,
		Member: jobKey,
	}).Err()
}

// GetFrontCount 返回某个任务前方排队数量（不在队列返回 -1）
func (s *queueService) GetFrontCount(ctx context.Context, jobID int) (int, error) {
	rank, err := s.redis.ZRank(ctx, QueueKey, strconv.Itoa(jobID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return -1, nil // 不在队列中
		}
		return 0, err
	}

	return int(rank), nil // rank 本身就是前方数量
}
