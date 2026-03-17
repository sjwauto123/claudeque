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
	"time"

	"go.uber.org/zap"
)

// jobService 任务服务实现
type jobService struct {
	jobRepo     repository.JobRepository
	queueSvc    QueueService
	gpuSvc      GpuService
	userRepo    repository.UserRepository
	execSvc     ExecService
	scheduler   Scheduler
	authService AuthService // 添加 authService
}

// NewJobService 创建任务服务
func NewJobService(jobRepo repository.JobRepository, queueSvc QueueService, gpuSvc GpuService, userRepo repository.UserRepository, execSvc ExecService, authService AuthService) JobService {
	return &jobService{
		jobRepo:     jobRepo,
		queueSvc:    queueSvc,
		gpuSvc:      gpuSvc,
		userRepo:    userRepo,
		execSvc:     execSvc,
		authService: authService,
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
	// 检查用户是否拥有 Root 权限
	isRoot, err := s.authService.HasSystemAccess(userID, AccessTypeFile)
	if err != nil {
		logger.Errorf("SubmitJob: 检查用户权限失败: userID=%d, err=%v", userID, err)
		return nil, fmt.Errorf("检查用户权限失败: %w", err)
	}

	// 检查远程文件路径是否存在
	exists, err := s.execSvc.FileExistsRemote(ctx, userID, req.FilePath, isRoot)
	if err != nil {
		logger.Errorf("SubmitJob: 检查远程文件失败: userID=%d, path=%s, err=%v", userID, req.FilePath, err)
		return nil, fmt.Errorf("检查任务文件失败: %w", err)
	}
	if !exists {
		logger.Infof("SubmitJob: 任务文件不存在: userID=%d, path=%s", userID, req.FilePath)
		return nil, errors.New(errors.CodeFileNotFound, "任务文件不存在")
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
		CondaEnv:    req.CondaEnv,
		GpuIDs:      gpuIDs,
		Status:      entity.JobStatusQueued,
	}

	if err := s.jobRepo.Create(job); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	enqueueErr := s.queueSvc.Enqueue(ctx, job.ID, priority)
	if enqueueErr != nil {
		if err := s.jobRepo.Delete(job.ID); err != nil {
			logger.Warn("回滚删除任务失败", zap.Int("job_id", job.ID), zap.Error(err))
		}
		return nil, fmt.Errorf("加入排队队列失败: %w", enqueueErr)
	}

	logger.Info("加入排队队列成功", zap.Int("job_id", job.ID)) // 这里不再打印 err，因为我们知道它是 nil
	return job, nil
}

// CancelJob 取消任务
func (s *jobService) CancelJob(ctx context.Context, jobID int, userID int) error {
	// 获取任务信息
	job, err := s.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 验证权限
	if job.UserId != userID {
		return errors.New(errors.CodeForbidden, "无权取消此任务")
	}

	// 只有排队中或等待显卡的任务可以取消
	if job.Status != entity.JobStatusQueued && job.Status != entity.JobStatusWaitingGpu {
		return errors.New(errors.CodeJobCannotCancel, "任务状态不允许取消")
	}

	// 更新任务状态
	if err := s.jobRepo.UpdateStatus(jobID, entity.JobStatusCancelled); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 从队列中移除
	if err := s.queueSvc.Remove(ctx, jobID); err != nil {
		logger.Warn("队列删除失败，回滚任务状态", zap.Int("job_id", jobID), zap.Error(err))

		if err := s.jobRepo.UpdateStatus(jobID, job.Status); err != nil {
			logger.Warn("回滚跟任务状态失败", zap.Int("job_id", job.ID), zap.Error(err))
		}

		return fmt.Errorf("从队列移除失败: %w", err)
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

func (s *jobService) ListCondaEnvs(ctx context.Context, userID int) ([]string, error) {
	return s.execSvc.ListCondaEnvs(ctx, userID)
}

func (s *jobService) SetScheduler(scheduler Scheduler) {
	s.scheduler = scheduler
}
