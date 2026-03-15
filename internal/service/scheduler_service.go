package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"

	"go.uber.org/zap"
)

// Scheduler 任务调度器
type scheduler struct {
	jobRepo     repository.JobRepository
	queueSvc    QueueService
	gpuSvc      GpuService
	processRepo repository.ProcessRepository
	procCache   repository.ProcessCacheRepository
	execSvc     ExecService

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
	pid     int // 存储进程ID
	jobID   int
	userID  int
	jobName string
	cardIDs []int
	done    chan struct{}
}

// NewScheduler 创建调度器
func NewScheduler(
	jobRepo repository.JobRepository,
	queueSvc QueueService,
	gpuSvc GpuService,
	processRepo repository.ProcessRepository,
	procCache repository.ProcessCacheRepository,
	execSvc ExecService,
) Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &scheduler{
		jobRepo:     jobRepo,
		queueSvc:    queueSvc,
		gpuSvc:      gpuSvc,
		processRepo: processRepo,
		procCache:   procCache,
		execSvc:     execSvc,
		ctx:         ctx,
		cancel:      cancel,
		runningJobs: make(map[int]*JobProcess),
	}
}

// Start 启动调度器
func (s *scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	s.isRunning = true

	// 同步恢复之前运行的任务
	s.recoverRunningJobs()

	s.wg.Add(1)
	go s.run()

	// 启动周期性同步
	s.wg.Add(1)
	go s.startAuditor()

	logger.Info("任务调度器已启动")
}

// startAuditor 启动同步器 goroutine
func (s *scheduler) startAuditor() {
	defer s.wg.Done()

	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("同步器停止")
			return
		case <-ticker.C:
			s.auditRunningJobs()
		}
	}
}

// Stop 停止调度器
func (s *scheduler) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	// 立即取消上下文，停止 Run() 循环中的 Ticker 触发
	s.cancel()

	// 等待所有goroutine退出（包括 Run 和所有的 MonitorJob）
	s.wg.Wait()

	logger.Info("任务调度器已停止")
}

// run 调度器主循环
func (s *scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
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
func (s *scheduler) processQueue() {
	// 检查调度器是否正在运行，如果正在停止，则不再处理新任务
	s.mu.RLock()
	if !s.isRunning {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

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
	logger.Warn("任务id为和状态为", zap.Int("job_id", job.ID), zap.Int("status", job.Status))
	// 检查任务状态
	if job.Status != entity.JobStatusQueued && job.Status != entity.JobStatusWaitingGpu {
		// 任务状态异常，从队列移除
		logger.Warn("任务状态异常，从队列移除", zap.Int("job_id", job.ID), zap.Int("status", job.Status))
		if err := s.queueSvc.Remove(s.ctx, item.JobID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", item.JobID))
		}
		return
	}

	// 解析任务需要的显卡 ID
	var cardIDs []int
	if job.GpuIDs != "" {
		idStrs := strings.Split(job.GpuIDs, ",")
		for _, s := range idStrs {
			id, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				logger.Warn("解析任务显卡ID失败", zap.Error(err), zap.Int("job_id", job.ID), zap.String("gpu_ids", job.GpuIDs))
				continue
			}
			cardIDs = append(cardIDs, id)
		}
	}

	if len(cardIDs) == 0 {
		logger.Warn("任务未指定显卡ID", zap.Int("job_id", job.ID))
		if err := s.queueSvc.Remove(s.ctx, item.JobID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return
	}

	// 检查显卡是否可用
	available, err := s.gpuSvc.CheckAvailable(s.ctx, cardIDs)
	if err != nil {
		logger.Error("检查显卡状态失败", zap.Error(err), zap.Ints("card_ids", cardIDs))
		return
	}

	if !available {
		// 显卡不可用，更新任务状态为等待显卡
		if job.Status != entity.JobStatusWaitingGpu {
			if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusWaitingGpu); err != nil {
				logger.Error("更新任务状态失败", zap.Error(err), zap.Int("job_id", job.ID))
			}
		}
		return
	}

	// 显卡可用，执行任务
	if err := s.executeJob(job, cardIDs); err != nil {
		logger.Error("启动任务失败", zap.Error(err), zap.Int("job_id", job.ID))
		// 如果启动失败，状态在 executeJob 内部已经处理，但需要从队列移除
		if err := s.queueSvc.Remove(s.ctx, item.JobID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", item.JobID))
		}
		return
	}

	// 启动成功，从队列移除
	if err := s.queueSvc.Remove(s.ctx, item.JobID); err != nil {
		logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", item.JobID))
	}
}

// executeJob 执行任务
func (s *scheduler) executeJob(job *entity.Job, cardIDs []int) error {
	logger.Info("开始执行任务", zap.Int("job_id", job.ID), zap.String("name", job.Name))

	// 占用显卡
	if err := s.gpuSvc.AcquireCards(s.ctx, cardIDs, job.ID); err != nil {
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
		// 移除队列
		if err := s.queueSvc.Remove(s.ctx, job.ID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return fmt.Errorf("任务脚本路径为空")
	}

	// 获取GPU
	cards, err := s.gpuSvc.GetGpuCardsByIDs(s.ctx, cardIDs)
	if err != nil {
		logger.Error("获取显卡详情失败", zap.Error(err), zap.Ints("card_ids", cardIDs))
		return fmt.Errorf("获取显卡详情失败: %w", err)
	}
	// GPU index
	gpuIndices := make([]string, len(cards))
	for i, card := range cards {
		gpuIndices[i] = strconv.Itoa(card.Index)
	}

	// 通过 SSH 后台执行，拿到 PID
	workdir := filepath.Dir(scriptPath)
	env := map[string]string{
		"CUDA_VISIBLE_DEVICES": strings.Join(gpuIndices, ","),
		"JOB_ID":               strconv.Itoa(job.ID),
		"PYTHONUNBUFFERED":     "1",
	}
	escPath := "'" + strings.ReplaceAll(scriptPath, "'", "'\\''") + "'"
	var cmdStr string
	if job.CondaEnv != "" {
		escEnv := "'" + strings.ReplaceAll(job.CondaEnv, "'", "'\\''") + "'"
		cmdStr = "source \"$(conda info --base)/etc/profile.d/conda.sh\" >/dev/null 2>&1 || true; conda activate " + escEnv + " && python -u " + escPath
	} else {
		cmdStr = "python -u " + escPath
	}
	pid, runErr := s.execSvc.RunBackgroundForUser(s.ctx, job.UserId, cmdStr, workdir, env)
	if runErr != nil || pid <= 0 {
		if err := s.gpuSvc.ReleaseCards(s.ctx, cardIDs); err != nil {
			logger.Warn("释放显卡失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
			logger.Warn("更新任务状态失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		if err := s.queueSvc.Remove(s.ctx, job.ID); err != nil {
			logger.Warn("从队列移除任务失败", zap.Error(err), zap.Int("job_id", job.ID))
		}
		return fmt.Errorf("启动训练脚本失败: %w", runErr)
	}
	logger.Info("任务路径", zap.String("job_file_path", job.FilePath))

	// 记录运行中的任务
	jp := &JobProcess{
		pid:     pid, // 存储 PID
		jobID:   job.ID,
		userID:  job.UserId,
		jobName: job.Name,
		cardIDs: cardIDs,
		done:    make(chan struct{}),
	}
	s.runningMu.Lock()
	s.runningJobs[jp.pid] = jp // 使用 jp.pid 作为键
	s.runningMu.Unlock()

	// 写入进程表
	for _, cardID := range cardIDs {
		process := &entity.Process{
			PID:    pid,
			CardID: cardID,
			JobID:  job.ID,
		}
		if err := s.processRepo.Create(process); err != nil {
			logger.Warn("写入进程表失败", zap.Error(err), zap.Int("pid", pid), zap.Int("card_id", cardID))
			return err
		}
	}

	// 写入进程缓存
	if err := s.procCache.CreatePid(s.ctx, pid, job.Name); err != nil {
		logger.Warn("写入进程缓存失败", zap.Error(err), zap.String("job_name", job.Name), zap.Int("pid", pid))
		return err
	}

	// 监控进程
	s.wg.Add(1)
	go s.monitorJob(jp)

	logger.Info("任务已启动",
		zap.Int("job_id", job.ID),
		zap.Int("pid", pid),
		zap.Ints("gpu_ids", cardIDs))

	return nil
}

// monitorJob 监控任务执行状态
func (s *scheduler) monitorJob(jp *JobProcess) {
	jobID := jp.jobID
	cardIDs := jp.cardIDs
	jobName := jp.jobName

	defer s.wg.Done()
	defer close(jp.done)
	defer func() {
		// 从运行列表中移除
		s.runningMu.Lock()
		delete(s.runningJobs, jp.pid) // 使用 jp.pid 作为键
		s.runningMu.Unlock()

		// 释放显卡和清理进程表使用 Background，确保在程序关闭时也能执行成功
		cleanupCtx := context.Background()

		// 删除进程缓存
		if err := s.procCache.DelPid(cleanupCtx, jp.pid); err != nil {
			logger.Warn("删除进程缓存失败", zap.Error(err), zap.Int("pid", jp.pid), zap.String("job_Name", jobName))
		}

		// 更新进程表，记录结束时间
		now := time.Now()
		processes, err := s.processRepo.FindActiveByJobID(jobID)
		if err == nil {
			for _, p := range processes {
				p.EndedAt = &now
				if err := s.processRepo.Update(&p); err != nil {
					logger.Warn("更新进程结束时间失败", zap.Error(err), zap.Int("job_id", jobID), zap.Int("pid", p.PID))
				}
			}
		} else {
			logger.Warn("查询活跃进程记录失败", zap.Error(err), zap.Int("job_id", jobID))
		}

		// 释放显卡
		if err := s.gpuSvc.ReleaseCards(cleanupCtx, cardIDs); err != nil {
			logger.Error("释放显卡失败", zap.Error(err), zap.Int("job_id", jobID), zap.Ints("card_ids", cardIDs))
		}
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			logger.Info("调度器停止，monitorJob 退出，任务进程继续运行", zap.Int("job_id", jobID), zap.Int("pid", jp.pid))
			return
		case <-ticker.C:
			running, runErr := s.execSvc.IsProcessRunning(s.ctx, jp.userID, jp.pid)
			if runErr != nil {
				logger.Warn("远程检查进程失败", zap.Error(runErr), zap.Int("job_id", jobID), zap.Int("pid", jp.pid))
			}
			if running {
				continue
			}
			job, err2 := s.jobRepo.GetByID(jobID)
			if err2 != nil || job == nil {
				return
			}
			code, readErr := s.execSvc.GetExitCode(s.ctx, jp.userID, jobID)
			status := entity.JobStatusCompleted
			if readErr != nil {
				status = entity.JobStatusFailed
				logger.Info("任务执行结束（未获取退出码文件）", zap.Int("job_id", jobID), zap.Error(readErr))
			} else {
				if code != 0 {
					status = entity.JobStatusFailed
					logger.Info("任务执行结束（异常退出）", zap.Int("job_id", jobID), zap.Int("exit_code", code))
				} else {
					status = entity.JobStatusCompleted
					logger.Info("任务执行成功", zap.Int("job_id", jobID))
				}
			}
			if err := s.jobRepo.UpdateStatus(jobID, status); err != nil {
				logger.Error("更新任务状态失败", zap.Error(err), zap.Int("job_id", jobID))
			}
			return
		}
	}
}

// recoverRunningJobs 恢复调度器启动前正在运行的任务
func (s *scheduler) recoverRunningJobs() {
	logger.Info("开始恢复正在运行的任务...")
	ctx := context.Background() // 使用 Background context 进行恢复操作

	// 查询数据库中所有状态为 JobStatusRunning 的任务
	runningJobsInDB, err := s.jobRepo.GetRunningJobsWithoutPagination(ctx)
	if err != nil {
		logger.Error("恢复任务时，查询数据库中运行中的任务失败", zap.Error(err))
		return
	}

	for _, job := range runningJobsInDB {
		logger.Info("发现数据库中运行中的任务", zap.Int("job_id", job.ID), zap.String("job_name", job.Name))

		// 查询该任务对应的活跃进程记录
		processes, err := s.processRepo.FindActiveByJobID(job.ID)
		if err != nil {
			logger.Error("恢复任务时，查询任务活跃进程失败", zap.Error(err), zap.Int("job_id", job.ID))
			continue
		}

		if len(processes) == 0 {
			logger.Warn("数据库中任务状态为运行中，但未找到活跃进程记录，将任务标记为失败", zap.Int("job_id", job.ID))
			handleMissingProcess(s, ctx, job, nil) // 进程记录不存在，传递 nil
			continue
		}

		// 遍历所有活跃进程记录
		for _, processRecord := range processes {
			pid := processRecord.PID

			// 检查进程是否存在（远程）
			running, checkErr := s.execSvc.IsProcessRunning(ctx, job.UserId, pid)
			if checkErr != nil {
				logger.Warn("远程检查进程失败", zap.Error(checkErr), zap.Int("job_id", job.ID), zap.Int("pid", pid))
			}
			if running {
				logger.Info("进程仍在运行，重新接管任务", zap.Int("job_id", job.ID), zap.Int("pid", pid))

				// 重新构建 JobProcess
				jp := &JobProcess{
					pid:     pid,
					jobID:   job.ID,
					userID:  job.UserId,
					jobName: job.Name,
					cardIDs: parseGpuIDs(job.GpuIDs),
					done:    make(chan struct{}),
				}

				// 添加到 runningJobs
				s.runningMu.Lock()
				s.runningJobs[jp.pid] = jp // 使用 jp.pid 作为键
				s.runningMu.Unlock()

				// 重新启动 monitorJob
				s.wg.Add(1)
				go s.monitorJob(jp)

				// 确保进程缓存存在
				if err := s.procCache.CreatePid(ctx, pid, job.Name); err != nil {
					logger.Warn("恢复时写入进程缓存失败", zap.Error(err), zap.String("job_name", job.Name), zap.Int("pid", pid))
				}

			} else {
				logger.Warn("进程已不存在，清理任务记录", zap.Int("job_id", job.ID), zap.Int("pid", pid))
				handleMissingProcess(s, ctx, job, &processRecord) // 传递 processRecord 的地址
			}
		}
	}

	logger.Info("正在运行的任务恢复完成。")
}

// auditRunningJobs 周期性同步运行中的任务，清理已不存在的进程
func (s *scheduler) auditRunningJobs() {
	logger.Info("开始周期性同步运行中的任务...")
	ctx := context.Background() // 使用 Background context 进行同步操作

	// 查询数据库中所有状态为 JobStatusRunning 的任务
	runningJobsInDB, err := s.jobRepo.GetRunningJobsWithoutPagination(ctx)
	if err != nil {
		logger.Error("同步任务时，查询数据库中运行中的任务失败", zap.Error(err))
		return
	}

	for _, job := range runningJobsInDB {
		// 查询该任务对应的活跃进程记录
		processes, err := s.processRepo.FindActiveByJobID(job.ID)
		if err != nil {
			logger.Error("同步任务时，查询任务活跃进程失败", zap.Error(err), zap.Int("job_id", job.ID))
			continue
		}

		if len(processes) == 0 {
			logger.Warn("同步发现数据库中任务状态为运行中，但未找到活跃进程记录，将任务标记为失败", zap.Int("job_id", job.ID))
			handleMissingProcess(s, ctx, job, nil) // 进程记录不存在，传递 nil
			continue
		}

		// 遍历所有活跃进程记录，确保处理一个任务的多个相关进程
		for _, processRecord := range processes {
			pid := processRecord.PID

			// 检查该进程是否在调度器的 runningJobs 列表中
			s.runningMu.RLock()
			_, isTracked := s.runningJobs[pid]
			s.runningMu.RUnlock()

			if isTracked {
				// 如果进程正在被调度器跟踪，则跳过，monitorJob 会处理其状态
				continue
			}

			// 检查进程是否存在（远程）
			running, checkErr := s.execSvc.IsProcessRunning(ctx, job.UserId, pid)
			if checkErr != nil {
				logger.Warn("远程检查进程失败", zap.Error(checkErr), zap.Int("job_id", job.ID), zap.Int("pid", pid))
				continue
			}
			if !running {
				logger.Warn("同步发现进程已不存在，清理任务记录", zap.Int("job_id", job.ID), zap.Int("pid", pid))
				handleMissingProcess(s, ctx, job, &processRecord) // 传递 processRecord 的地址
			}
		}
	}

	logger.Info("周期性同步运行中的任务完成。")
}

// isProcessRunning 检查指定 PID 的进程是否仍在运行
//func isProcessRunning(pid int) bool {
//	process, err := os.FindProcess(pid)
//	if err != nil {
//		return false
//	}
//	err = process.Signal(syscall.Signal(0))
//	if err == nil {
//		return true
//	}
//	if errors.Is(err, syscall.EPERM) {
//		return true
//	}
//	return false
//}

// parseGpuIDs 辅助函数，解析 GPU ID 字符串
func parseGpuIDs(gpuIDsStr string) []int {
	var cardIDs []int
	if gpuIDsStr != "" {
		idStrs := strings.Split(gpuIDsStr, ",")
		for _, s := range idStrs {
			id, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				logger.Warn("解析任务显卡ID失败", zap.Error(err), zap.String("gpu_ids", gpuIDsStr))
				continue
			}
			cardIDs = append(cardIDs, id)
		}
	}
	return cardIDs
}

// handleMissingProcess 处理进程不存在的情况
func handleMissingProcess(s *scheduler, ctx context.Context, job *entity.Job, processRecord *entity.Process) {
	// 更新任务状态为失败
	if err := s.jobRepo.UpdateStatus(job.ID, entity.JobStatusFailed); err != nil {
		logger.Error("更新任务状态为失败失败", zap.Error(err), zap.Int("job_id", job.ID))
	}

	// 清理 processRepo 记录
	if processRecord != nil {
		now := time.Now()
		processRecord.EndedAt = &now
		if err := s.processRepo.Update(processRecord); err != nil {
			logger.Error("更新进程结束时间失败", zap.Error(err), zap.Int("job_id", job.ID), zap.Int("pid", processRecord.PID))
		}
	}

	// 清理 procCache 缓存（有进程记录时使用 PID 删除）
	if processRecord != nil {
		if err := s.procCache.DelPid(ctx, processRecord.PID); err != nil {
			logger.Warn("恢复时删除进程缓存失败", zap.Error(err), zap.Int("pid", processRecord.PID), zap.String("job_name", job.Name))
		}
	}

	// 释放显卡
	cardIDs := parseGpuIDs(job.GpuIDs)
	if err := s.gpuSvc.ReleaseCards(ctx, cardIDs); err != nil {
		logger.Error("恢复时释放显卡失败", zap.Error(err), zap.Int("job_id", job.ID), zap.Ints("card_ids", cardIDs))
	}
}
