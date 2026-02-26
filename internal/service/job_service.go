package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"os"
	"time"

	"go.uber.org/zap"
)

// jobService 任务服务实现
type jobService struct {
	jobRepo  repository.JobRepository
	queueSvc QueueService
	gpuSvc   GpuService
	userRepo repository.UserRepository
}

// NewJobService 创建任务服务
func NewJobService(jobRepo repository.JobRepository, queueSvc QueueService, gpuSvc GpuService, userRepo repository.UserRepository) JobService {
	return &jobService{
		jobRepo:  jobRepo,
		queueSvc: queueSvc,
		gpuSvc:   gpuSvc,
		userRepo: userRepo,
	}
}

// GetJobList 获取任务列表
func (s *jobService) GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error) {
	if req.PageSize <= 0 {
		req.PageSize = 5
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	return s.jobRepo.GetJobList(req, startTime, endTime, userID)
}

// GetWaitJobList 获取正在排队的任务列表
func (s *jobService) GetWaitJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error) {
	if req.PageSize <= 0 {
		req.PageSize = 5
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	return s.jobRepo.GetWaitJobList(req, startTime, endTime, userID)
}

// SubmitJob 提交任务
func (s *jobService) SubmitJob(ctx context.Context, req request.SubmitJobRequest, userID int) (*entity.Job, error) {
	// 检查文件路径是否存在
	if _, err := os.Stat(req.FilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeFileNotFound, "任务文件不存在: "+req.FilePath)
		}
		return nil, fmt.Errorf("检查任务文件失败: %w", err)
	}

	// 获取用户优先级
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	priority := entity.JobPriorityLow
	if user != nil && user.Priority > 0 {
		priority = user.Priority
	}

	// 转换 GPU ID 数组为逗号分隔字符串
	gpuIDStrs := make([]string, len(req.GpuIDs))
	for i, id := range req.GpuIDs {
		gpuIDStrs[i] = strconv.Itoa(id)
	}
	gpuIDs := strings.Join(gpuIDStrs, ",")

	// 创建任务记录
	job := &entity.Job{
		Name:        req.Name,
		Description: req.Description,
		UserId:      userID,
		FilePath:    req.FilePath,
		GpuIDs:      gpuIDs,
		Status:      entity.JobStatusPending,
	}

	if err := s.jobRepo.Create(job); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// 将任务加入排队队列
	if err := s.queueSvc.Enqueue(ctx, job.ID, priority); err != nil {
		// 如果入队失败，更新任务状态为失败
		if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
			logger.Warn("更新任务状态失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return nil, fmt.Errorf("加入排队队列失败: %w", err)
	}

	// 更新任务状态为排队中
	if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusQueued); err != nil {
		return nil, fmt.Errorf("更新任务状态失败: %w", err)
	}
	return job, nil
}

// CancelJob 取消任务
func (s *jobService) CancelJob(ctx context.Context, jobID int, userID int) error {
	// 获取任务信息
	job, err := s.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}
	if job == nil {
		return errors.New(errors.CodeJobNotFound, "任务不存在")
	}

	// 验证权限
	if job.UserId != userID {
		return errors.New(errors.CodeForbidden, "无权取消此任务")
	}

	// 只有排队中或等待显卡的任务可以取消
	if job.Status != entity.JobStatusQueued && job.Status != entity.JobStatusWaitingGpu {
		return errors.New(errors.CodeJobCannotCancel, "任务状态不允许取消")
	}

	// 从队列中移除
	if err := s.queueSvc.Remove(ctx, jobID); err != nil {
		return fmt.Errorf("从队列移除失败: %w", err)
	}

	// 更新任务状态
	if err := s.jobRepo.UpdateStatus(jobID, entity.JobStatusCancelled); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	return nil
}

func (s *jobService) EnrichJobList(ctx context.Context, jobs []response.JobResponse) error {
	if s.queueSvc == nil {
		return nil
	}

	now := time.Now()

	for i := range jobs {
		jobID := jobs[i].ID

		front, err := s.queueSvc.GetFrontCount(ctx, jobID)
		if err != nil {
			return err
		}

		if front >= 0 {
			jobs[i].Count = front
			jobs[i].WaitTime = strconv.Itoa(int(now.Sub(jobs[i].CreatedAt).Seconds()))
		} else {
			jobs[i].Count = -1
		}
	}

	return nil
}

func (s *jobService) GetJobByID(jobID int) (*entity.Job, error) {
	return s.jobRepo.GetByID(jobID)
}

func (s *jobService) GetStats() (*response.JobStatsResponse, error) {
	return s.jobRepo.GetStats()
}
