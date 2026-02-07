package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Scheduler 任务调度器
type Scheduler struct {
	db          *gorm.DB
	redis       *redis.Client
	jobRepo     repository.JobRepository
	queueSvc    QueueService
	gpuSvc      *GpuService
	processRepo repository.ProcessRepository

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.RWMutex

	// 跟踪正在运行的任务进程
	runningJobs map[int]*JobProcess
	runningMu   sync.RWMutex
}

// JobProcess 任务进程信息
type JobProcess struct {
	cmd     *exec.Cmd
	jobID   int
	cardIDs []int
}

// NewScheduler 创建调度器
func NewScheduler(
	db *gorm.DB,
	redis *redis.Client,
	jobRepo repository.JobRepository,
	queueSvc QueueService,
	gpuSvc *GpuService,
	processRepo repository.ProcessRepository,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		db:          db,
		redis:       redis,
		jobRepo:     jobRepo,
		queueSvc:    queueSvc,
		gpuSvc:      gpuSvc,
		processRepo: processRepo,
		ctx:         ctx,
		cancel:      cancel,
		runningJobs: make(map[int]*JobProcess),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	s.isRunning = true

	s.wg.Add(1)
	go s.run()

	logger.Info("任务调度器已启动")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	// 取消上下文
	s.cancel()

	// 等待所有goroutine退出
	s.wg.Wait()

	// 终止所有正在运行的任务
	s.terminateAllRunningJobs()

	logger.Info("任务调度器已停止")
}

// terminateAllRunningJobs 终止所有正在运行的任务
func (s *Scheduler) terminateAllRunningJobs() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for jobID, jp := range s.runningJobs {
		logger.Info("终止任务", zap.Int("job_id", jobID))
		if jp.cmd.Process != nil {
			if err := jp.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				logger.Warn("发送终止信号失败", zap.Error(err), zap.Int("job_id", jobID))
			}
		}
		delete(s.runningJobs, jobID)
	}
}

// run 调度器主循环
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processQueue()
		}
	}
}

// processQueue 处理排队队列
func (s *Scheduler) processQueue() {
	// 获取队列头部的任务
	item, err := s.queueSvc.Peek(s.ctx)
	if err != nil {
		logger.Error("获取队列头部失败", zap.Error(err))
		return
	}

	if item == nil {
		return // 队列为空
	}

	// 获取任务详情
	job, err := s.jobRepo.GetByID(item.JobID)
	if err != nil {
		logger.Error("获取任务详情失败", zap.Error(err), zap.Int("job_id", item.JobID))
		return
	}

	if job == nil {
		// 任务不存在，从队列移除
		if err := s.queueSvc.Remove(s.ctx, item.JobID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", item.JobID))
		}
		return
	}

	// 检查任务状态
	if job.Status != entity.JobStatusQueued && job.Status != entity.JobStatusWaitingGpu {
		// 任务状态异常，从队列移除
		logger.Warn("任务状态异常，从队列移除",
			zap.Int("job_id", job.ID),
			zap.Int("status", job.Status))
		if err := s.queueSvc.Remove(s.ctx, item.JobID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", item.JobID))
		}
		return
	}

	// 检查是否有足够的空闲显卡
	idleCount, err := s.gpuSvc.GetIdleCount(s.ctx)
	if err != nil {
		logger.Error("获取空闲显卡数量失败", zap.Error(err))
		return
	}

	if idleCount < job.GpuCount {
		// 显卡不足，更新任务状态为等待显卡
		if job.Status != entity.JobStatusWaitingGpu {
			if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusWaitingGpu); err != nil {
				logger.Error("更新任务状态失败", zap.Error(err))
			}
		}
		logger.Debug("等待足够显卡",
			zap.Int("job_id", job.ID),
			zap.Int("need", job.GpuCount),
			zap.Int("idle", idleCount))
		return
	}

	// 执行任务
	if err := s.executeJob(job); err != nil {
		logger.Error("执行任务失败", zap.Error(err), zap.Int("job_id", job.ID))
		// 更新任务状态为失败
		if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
			logger.Error("更新任务状态失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		// 从队列移除
		if err := s.queueSvc.Remove(s.ctx, job.ID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return
	}

	// 从队列移除
	if err := s.queueSvc.Remove(s.ctx, job.ID); err != nil {
		logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", job.ID))
	}
}

// executeJob 执行任务
func (s *Scheduler) executeJob(job *entity.Job) error {
	logger.Info("开始执行任务", zap.Int("job_id", job.ID), zap.String("name", job.Name))

	// 占用显卡
	cardIDs, err := s.gpuSvc.AcquireCards(s.ctx, job.GpuCount, job.ID)
	if err != nil {
		return fmt.Errorf("占用显卡失败: %w", err)
	}

	// 更新任务状态为执行中
	if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusRunning); err != nil {
		if err := s.gpuSvc.ReleaseCards(s.ctx, cardIDs); err != nil {
			logger.Warn("释放显卡失败", zap.Error(err), zap.Int("job_id", job.ID), zap.Ints("card_ids", cardIDs))
		}
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 构建训练脚本路径
	scriptPath := job.FilePath
	if scriptPath == "" {
		if err := s.gpuSvc.ReleaseCards(s.ctx, cardIDs); err != nil {
			logger.Warn("释放显卡失败", zap.Error(err), zap.Int("job_id", job.ID), zap.Ints("card_ids", cardIDs))
		}
		if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
			logger.Warn("更新任务状态失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return fmt.Errorf("任务脚本路径为空")
	}

	// 构建命令
	cmd := exec.Command("python", "-u", scriptPath)
	cmd.Dir = filepath.Dir(scriptPath)

	// 设置环境变量，指定使用的GPU
	gpuIDs := make([]string, len(cardIDs))
	for i, id := range cardIDs {
		gpuIDs[i] = strconv.Itoa(id - 1)
	}
	cmd.Env = append(os.Environ(),
		"CUDA_VISIBLE_DEVICES="+strings.Join(gpuIDs, ","),
		"JOB_ID="+strconv.Itoa(job.ID),
		"PYTHONUNBUFFERED=1",
	)

	// 启动任务
	if err := cmd.Start(); err != nil {
		if err := s.gpuSvc.ReleaseCards(s.ctx, cardIDs); err != nil {
			logger.Warn("释放显卡失败", zap.Error(err), zap.Int("job_id", job.ID), zap.Ints("card_ids", cardIDs))
		}
		if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
			logger.Warn("更新任务状态失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return fmt.Errorf("启动训练脚本失败: %w", err)
	}

	// 记录运行中的任务
	jp := &JobProcess{
		cmd:     cmd,
		jobID:   job.ID,
		cardIDs: cardIDs,
	}
	s.runningMu.Lock()
	s.runningJobs[job.ID] = jp
	s.runningMu.Unlock()

	// 写入进程表
	pid := cmd.Process.Pid
	for _, cardID := range cardIDs {
		process := &entity.Process{
			PID:    pid,
			CardID: cardID,
			JobID:  job.ID,
		}
		if err := s.processRepo.Create(process); err != nil {
			logger.Warn("写入进程表失败", zap.Error(err), zap.Int("pid", pid), zap.Int("card_id", cardID))
		}
	}

	// 启动goroutine监控任务执行状态
	go s.monitorJob(jp)

	logger.Info("任务已启动",
		zap.Int("job_id", job.ID),
		zap.String("pid", strconv.Itoa(cmd.Process.Pid)),
		zap.Ints("gpu_ids", cardIDs))

	return nil
}

// monitorJob 监控任务执行状态
func (s *Scheduler) monitorJob(jp *JobProcess) {
	jobID := jp.jobID
	cardIDs := jp.cardIDs

	defer func() {
		// 从运行列表中移除
		s.runningMu.Lock()
		delete(s.runningJobs, jobID)
		s.runningMu.Unlock()

		// 从进程表中删除
		if err := s.processRepo.DeleteByJobID(jobID); err != nil {
			logger.Warn("删除进程记录失败", zap.Error(err), zap.Int("job_id", jobID))
		}

		// 释放显卡
		if err := s.gpuSvc.ReleaseCards(s.ctx, cardIDs); err != nil {
			logger.Error("释放显卡失败",
				zap.Error(err),
				zap.Int("job_id", jobID),
				zap.Ints("card_ids", cardIDs))
		}
	}()

	// 等待命令执行完成
	err := jp.cmd.Wait()

	// 获取任务详情
	job, err2 := s.jobRepo.GetByID(jobID)
	if err2 != nil {
		logger.Error("获取任务详情失败", zap.Error(err2), zap.Int("job_id", jobID))
		return
	}

	if job == nil {
		return
	}

	// 检查进程退出码
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	// 更新任务状态
	if exitCode != 0 {
		// 任务执行失败
		logger.Error("任务执行失败",
			zap.Error(err),
			zap.Int("job_id", jobID),
			zap.Int("exit_code", exitCode),
			zap.String("name", job.Name))

		if err := s.jobRepo.UpdateStatus(jobID, entity.JobStatusFailed); err != nil {
			logger.Warn("更新任务状态失败", zap.Error(err), zap.Int("job_id", jobID))
		}
	} else {
		// 任务执行成功
		logger.Info("任务执行成功",
			zap.Int("job_id", jobID),
			zap.String("name", job.Name))

		if err := s.jobRepo.UpdateStatus(jobID, entity.JobStatusCompleted); err != nil {
			logger.Warn("更新任务状态失败", zap.Error(err), zap.Int("job_id", jobID))
		}
	}

	// 尝试调度下一个任务
	go s.processQueue()
}
