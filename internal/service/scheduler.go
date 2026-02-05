package service

import (
	"context"
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
	db       *gorm.DB
	redis    *redis.Client
	jobRepo  repository.JobRepository
	queueSvc QueueService
	gpuSvc   *GpuService

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.RWMutex

	// 跟踪正在运行的任务进程
	runningJobs map[uint]*JobProcess
	runningMu   sync.RWMutex

	// 任务日志目录
	logDir string
}

// JobProcess 任务进程信息
type JobProcess struct {
	cmd     *exec.Cmd
	logFile *os.File
	logPath string
	jobID   uint
	cardIDs []uint
}

// NewScheduler 创建调度器
func NewScheduler(
	db *gorm.DB,
	redis *redis.Client,
	jobRepo repository.JobRepository,
	queueSvc QueueService,
	gpuSvc *GpuService,
	logDir string,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	if logDir == "" {
		logDir = "./logs/jobs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		logger.Error("创建任务日志目录失败", zap.Error(err), zap.String("dir", logDir))
	}
	return &Scheduler{
		db:          db,
		redis:       redis,
		jobRepo:     jobRepo,
		queueSvc:    queueSvc,
		gpuSvc:      gpuSvc,
		ctx:         ctx,
		cancel:      cancel,
		runningJobs: make(map[uint]*JobProcess),
		logDir:      logDir,
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
		logger.Info("终止任务", zap.Uint("job_id", jobID))
		if jp.cmd.Process != nil {
			_ = jp.cmd.Process.Signal(syscall.SIGTERM)
		}
		if jp.logFile != nil {
			_ = jp.logFile.Close()
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
		logger.Error("获取任务详情失败", zap.Error(err), zap.Uint("job_id", item.JobID))
		return
	}

	if job == nil {
		// 任务不存在，从队列移除
		_ = s.queueSvc.Remove(s.ctx, item.JobID)
		return
	}

	// 检查任务状态
	if job.Status != entity.JobStatusQueued && job.Status != entity.JobStatusWaitingGpu {
		// 任务状态异常，从队列移除
		logger.Warn("任务状态异常，从队列移除",
			zap.Uint("job_id", job.ID),
			zap.Int("status", job.Status))
		_ = s.queueSvc.Remove(s.ctx, item.JobID)
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
			zap.Uint("job_id", job.ID),
			zap.Int("need", job.GpuCount),
			zap.Int("idle", idleCount))
		return
	}

	// 执行任务
	if err := s.executeJob(job); err != nil {
		logger.Error("执行任务失败", zap.Error(err), zap.Uint("job_id", job.ID))
		// 更新任务状态为失败
		if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
			logger.Error("更新任务状态失败", zap.Error(err))
		}
		// 从队列移除
		_ = s.queueSvc.Remove(s.ctx, job.ID)
		return
	}

	// 从队列移除
	_ = s.queueSvc.Remove(s.ctx, job.ID)
}

// executeJob 执行任务
func (s *Scheduler) executeJob(job *entity.Job) error {
	logger.Info("开始执行任务", zap.Uint("job_id", job.ID), zap.String("name", job.Name))

	// 1. 占用显卡
	cardIDs, err := s.gpuSvc.AcquireCards(s.ctx, job.GpuCount, job.ID)
	if err != nil {
		return fmt.Errorf("占用显卡失败: %w", err)
	}

	// 2. 更新任务状态为执行中
	if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusRunning); err != nil {
		_ = s.gpuSvc.ReleaseCards(s.ctx, cardIDs)
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 3. 创建日志文件
	logFileName := fmt.Sprintf("job_%d_%s.log", job.ID, time.Now().Format("20060102_150405"))
	logPath := filepath.Join(s.logDir, logFileName)
	logFile, err := os.Create(logPath)
	if err != nil {
		_ = s.gpuSvc.ReleaseCards(s.ctx, cardIDs)
		_ = s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed)
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 4. 构建训练脚本路径
	scriptPath := job.FilePath
	if scriptPath == "" {
		_ = s.gpuSvc.ReleaseCards(s.ctx, cardIDs)
		_ = s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed)
		_ = logFile.Close()
		return fmt.Errorf("任务脚本路径为空")
	}

	// 5. 构建命令
	cmd := exec.Command("python", "-u", scriptPath)
	cmd.Dir = filepath.Dir(scriptPath)

	// 设置环境变量，指定使用的GPU
	gpuIDs := make([]string, len(cardIDs))
	for i, id := range cardIDs {
		gpuIDs[i] = strconv.Itoa(int(id - 1))
	}
	cmd.Env = append(os.Environ(),
		"CUDA_VISIBLE_DEVICES="+strings.Join(gpuIDs, ","),
		"JOB_ID="+strconv.Itoa(int(job.ID)),
		"PYTHONUNBUFFERED=1",
	)

	// 6. 将输出重定向到日志文件
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 7. 启动任务
	if err := cmd.Start(); err != nil {
		_ = s.gpuSvc.ReleaseCards(s.ctx, cardIDs)
		_ = s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed)
		_ = logFile.Close()
		return fmt.Errorf("启动训练脚本失败: %w", err)
	}

	// 8. 记录运行中的任务
	jp := &JobProcess{
		cmd:     cmd,
		logFile: logFile,
		logPath: logPath,
		jobID:   job.ID,
		cardIDs: cardIDs,
	}
	s.runningMu.Lock()
	s.runningJobs[job.ID] = jp
	s.runningMu.Unlock()

	// 9. 更新任务的日志文件路径
	if err := s.jobRepo.UpdateLogPath(job.ID, logPath); err != nil {
		logger.Warn("更新任务日志路径失败", zap.Error(err), zap.Uint("job_id", job.ID))
	}

	// 10. 启动goroutine监控任务执行状态
	go s.monitorJob(jp)

	logger.Info("任务已启动",
		zap.Uint("job_id", job.ID),
		zap.String("pid", strconv.Itoa(cmd.Process.Pid)),
		zap.Uints("gpu_ids", cardIDs),
		zap.String("log_path", logPath))

	return nil
}

// monitorJob 监控任务执行状态
func (s *Scheduler) monitorJob(jp *JobProcess) {
	jobID := jp.jobID
	cardIDs := jp.cardIDs

	defer func() {
		// 关闭日志文件
		if jp.logFile != nil {
			_ = jp.logFile.Close()
		}

		// 从运行列表中移除
		s.runningMu.Lock()
		delete(s.runningJobs, jobID)
		s.runningMu.Unlock()

		// 释放显卡
		if err := s.gpuSvc.ReleaseCards(s.ctx, cardIDs); err != nil {
			logger.Error("释放显卡失败",
				zap.Error(err),
				zap.Uint("job_id", jobID),
				zap.Uints("card_ids", cardIDs))
		}
	}()

	// 等待命令执行完成
	err := jp.cmd.Wait()

	// 获取任务详情
	job, err2 := s.jobRepo.GetByID(jobID)
	if err2 != nil {
		logger.Error("获取任务详情失败", zap.Error(err2), zap.Uint("job_id", jobID))
		return
	}

	if job == nil {
		return
	}

	// 检查进程退出码
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
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
			zap.Uint("job_id", jobID),
			zap.Int("exit_code", exitCode),
			zap.String("name", job.Name),
			zap.String("log_path", jp.logPath))

		// 读取错误信息（日志文件最后几行）
		errorMsg := s.readLastLogLines(jp.logPath, 10)
		if errorMsg == "" {
			errorMsg = fmt.Sprintf("进程异常退出，退出码: %d", exitCode)
		}

		// 更新任务状态和错误信息
		updates := map[string]interface{}{
			"status":     entity.JobStatusFailed,
			"result_msg": errorMsg,
		}
		_ = s.db.Model(&entity.Job{}).Where("id = ?", jobID).Updates(updates)
	} else {
		// 任务执行成功
		logger.Info("任务执行成功",
			zap.Uint("job_id", jobID),
			zap.String("name", job.Name),
			zap.String("log_path", jp.logPath))

		_ = s.jobRepo.UpdateStatus(jobID, entity.JobStatusCompleted)
	}

	// 尝试调度下一个任务
	go s.processQueue()
}

// readLastLogLines 读取日志文件最后N行
func (s *Scheduler) readLastLogLines(logPath string, maxLines int) string {
	if logPath == "" {
		return ""
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return ""
	}

	// 移除空行
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// 取最后maxLines行
	start := len(nonEmptyLines) - maxLines
	if start < 0 {
		start = 0
	}

	return strings.Join(nonEmptyLines[start:], "\n")
}

// CancelRunningJob 取消正在运行的任务
func (s *Scheduler) CancelRunningJob(jobID uint) error {
	s.runningMu.Lock()
	jp, exists := s.runningJobs[jobID]
	s.runningMu.Unlock()

	if !exists {
		return fmt.Errorf("任务%d不在运行中", jobID)
	}

	if jp.cmd.Process != nil {
		if err := jp.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("终止进程失败: %w", err)
		}
	}

	return nil
}

// IsRunning 检查调度器是否在运行
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}
