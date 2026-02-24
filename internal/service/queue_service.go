package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"

	"github.com/redis/go-redis/v9"
)

const (
	// PriorityGap 优先级基数
	PriorityGap = int(1_000_000_000)
)

type queueService struct {
	queueRepo repository.QueueRepository
	jobRepo   repository.JobRepository
}

func NewQueueService(queueRepo repository.QueueRepository, jobRepo repository.JobRepository) QueueService {
	return &queueService{queueRepo: queueRepo, jobRepo: jobRepo}
}

// Enqueue 入队
func (s *queueService) Enqueue(ctx context.Context, jobID int, priority int) error {
	seq, err := s.queueRepo.NextSeq(ctx)
	if err != nil {
		return err
	}

	base := priority * PriorityGap

	score := float64(base + seq)

	return s.queueRepo.Add(ctx, jobID, score)
}

// Peek 获取队列第一个任务的信息
func (s *queueService) Peek(ctx context.Context) (*entity.Item, error) {
	jobID, score, err := s.queueRepo.Peek(ctx)
	if err != nil {
		return nil, err
	}

	if jobID == 0 {
		return nil, nil
	}

	return &entity.Item{
		JobID: jobID,
		Score: int(score),
	}, nil
}

// Remove 出队
func (s *queueService) Remove(ctx context.Context, jobID int) error {
	return s.queueRepo.Remove(ctx, jobID)
}

// GetQueuePage 分页查询，按条件检索，返回完整排队任务信息
func (s *queueService) GetQueuePage(ctx context.Context, req request.QueueListRequest, startTime, endTime time.Time) ([]response.QueueJobResponse, int64, int, int, error) {
	members, err := s.queueRepo.Range(ctx, 0, -1)
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
		orderedJobIDs = append(orderedJobIDs, jobID)
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
		rank, err := s.queueRepo.Rank(ctx, row.JobID)
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

	return list, int64(total), req.Page, req.PageSize, nil
}

// MoveBefore 更新排队
func (s *queueService) MoveBefore(ctx context.Context, jobID int, beforeJobID int) error {
	//  获取移动前的队首任务
	oldHeadID, _, err := s.queueRepo.Peek(ctx)
	if err != nil {
		return err
	}
	// 计算新分数
	rank, err := s.queueRepo.Rank(ctx, beforeJobID)
	if err != nil {
		return err
	}
	beforeScore, err := s.queueRepo.Score(ctx, beforeJobID)
	if err != nil {
		return err
	}

	var newScore float64
	if rank == 0 {
		newScore = beforeScore - float64(PriorityGap)
	} else {
		prevs, err := s.queueRepo.RangeWithScores(ctx, rank-1, rank-1)
		if err != nil {
			return err
		}
		prevScore := prevs[0].Score
		newScore = (prevScore + beforeScore) / 2
	}
	// 执行移动操作
	if err := s.queueRepo.Add(ctx, jobID, newScore); err != nil {
		return err
	}
	// 如果任务被移出了队首，或者之前的队首被挤到了后面,我们需要将不再处于队首且状态为 "等待显卡" 的任务重置为 "排队中"
	if oldHeadID != 0 {
		s.fixJobStatusIfMovedFromHead(ctx, oldHeadID)
	}
	// 检查被移动的任务本身
	s.fixJobStatusIfMovedFromHead(ctx, jobID)
	return nil
}

// fixJobStatusIfMovedFromHead 如果任务不再是队首，将其状态从 "等待显卡" 重置为 "排队中"
func (s *queueService) fixJobStatusIfMovedFromHead(ctx context.Context, jobID int) {
	rank, err := s.queueRepo.Rank(ctx, jobID)
	if err != nil {
		return // 不在队列中或 Redis 错误
	}
	// 如果不在队首
	if rank > 0 {
		job, err := s.jobRepo.GetByID(jobID)
		if err == nil && job != nil && job.Status == entity.JobStatusWaitingGpu {
			// 重置为排队中，让调度器在它再次到达队首时重新触发资源检查
			err = s.jobRepo.UpdateStatus(jobID, entity.JobStatusQueued)
			if err != nil {
				return
			}
		}
	}
}

// GetFrontCount 返回某个任务前方排队数量
func (s *queueService) GetFrontCount(ctx context.Context, jobID int) (int, error) {
	rank, err := s.queueRepo.Rank(ctx, jobID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return -1, nil // 不在队列中
		}
		return 0, err
	}

	return int(rank), nil // rank 本身就是前方数量
}
