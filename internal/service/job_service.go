package service

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"time"
)

// jobService 任务服务实现
type jobService struct {
	jobRepo  repository.JobRepository
	queueSvc QueueService
	gpuSvc   *GpuService
	userRepo repository.UserRepository
}

// NewJobService 创建任务服务
func NewJobService(jobRepo repository.JobRepository, queueSvc QueueService, gpuSvc *GpuService, userRepo repository.UserRepository) JobService {
	return &jobService{
		jobRepo:  jobRepo,
		queueSvc: queueSvc,
		gpuSvc:   gpuSvc,
		userRepo: userRepo,
	}
}

// GetJobList 获取任务列表
func (s *jobService) GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID uint) ([]response.JobResponse, int64, int, int, error) {
	if req.PageSize <= 0 {
		req.PageSize = 5
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	return s.jobRepo.GetJobList(req, startTime, endTime, userID)
}

// SubmitJob 提交任务
func (s *jobService) SubmitJob(ctx context.Context, req request.SubmitJobRequest, userID uint) (*entity.Job, error) {
	// 获取用户优先级
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	priority := entity.JobPriorityLow
	if user != nil && user.Priority > 0 {
		priority = user.Priority
	}

	// 创建任务记录
	job := &entity.Job{
		Name:        req.Name,
		Description: req.Description,
		UserId:      userID,
		FilePath:    req.FilePath,
		GpuCount:    req.GpuCount,
		//Priority:    priority,
		Status: entity.JobStatusPending,
	}

	if err := s.jobRepo.Create(job); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// 将任务加入排队队列
	if err := s.queueSvc.Enqueue(ctx, job.ID, priority); err != nil {
		// 如果入队失败，更新任务状态为失败
		_ = s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed)
		return nil, fmt.Errorf("加入排队队列失败: %w", err)
	}

	// 更新任务状态为排队中
	if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusQueued); err != nil {
		return nil, fmt.Errorf("更新任务状态失败: %w", err)
	}

	return job, nil
}

// CancelJob 取消任务
func (s *jobService) CancelJob(ctx context.Context, jobID uint, userID uint) error {
	// 获取任务信息
	job, err := s.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	if job == nil {
		return fmt.Errorf("任务不存在")
	}

	// 验证权限
	if job.UserId != userID {
		return fmt.Errorf("无权取消此任务")
	}

	// 只有排队中或等待显卡的任务可以取消
	if job.Status != entity.JobStatusQueued && job.Status != entity.JobStatusWaitingGpu {
		return fmt.Errorf("任务状态不允许取消")
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
		jobID := uint(jobs[i].ID)

		front, err := s.queueSvc.GetFrontCount(ctx, jobID)
		if err != nil {
			return err
		}

		if front >= 0 {
			jobs[i].Count = front
			jobs[i].WaitTime = strconv.FormatInt(int64(now.Sub(jobs[i].CreatedAt).Seconds()), 10)
		} else {
			jobs[i].Count = -1
		}
	}

	return nil
}

func (s *jobService) GetJobLog(ctx context.Context, jobID uint, userID uint) (string, error) {
	// 获取任务信息
	job, err := s.jobRepo.GetByID(jobID)
	if err != nil {
		return "", fmt.Errorf("获取任务失败: %w", err)
	}

	if job == nil {
		return "", fmt.Errorf("任务不存在")
	}

	// 验证权限
	if job.UserId != userID {
		return "", fmt.Errorf("无权查看此任务")
	}

	// 读取日志文件
	if job.LogPath == "" {
		return "", fmt.Errorf("任务日志不存在")
	}

	content, err := readLogFile(job.LogPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	return content, nil
}

func readLogFile(logPath string) (string, error) {
	if logPath == "" {
		return "", fmt.Errorf("日志路径为空")
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
