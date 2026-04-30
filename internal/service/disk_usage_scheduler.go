package service

import (
	"context"
	"sync"
	"time"

	"cloudque/pkg/logger"

	"go.uber.org/zap"
)

// DiskUsageScheduler 磁盘使用情况定时任务调度器
type DiskUsageScheduler struct {
	fileSvc FileService

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.RWMutex
	taskMu    sync.Mutex // 任务执行互斥锁
}

// NewDiskUsageScheduler 创建磁盘使用情况调度器
func NewDiskUsageScheduler(fileSvc FileService) *DiskUsageScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &DiskUsageScheduler{
		fileSvc:   fileSvc,
		ctx:       ctx,
		cancel:    cancel,
		isRunning: false,
	}
}

// Start 启动调度器
func (d *DiskUsageScheduler) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isRunning {
		logger.Info("磁盘使用情况调度器已在运行中")
		return
	}

	d.isRunning = true

	d.wg.Add(1)
	go d.run()

	logger.Info("磁盘使用情况调度器已启动")
}

// run 调度器主循环
func (d *DiskUsageScheduler) run() {
	defer d.wg.Done()

	// 启动时立即执行一次（异步执行）
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.executeTask()
	}()

	// 计算下次执行时间（每天凌晨 3 点）
	scheduleNext := func() time.Duration {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		if now.After(next) {
			// 如果已经过了今天凌晨 3 点，调度到明天凌晨 3 点
			next = next.Add(24 * time.Hour)
		}
		return next.Sub(now)
	}

	// 计算初始延迟
	initialDelay := scheduleNext()
	logger.Infof("磁盘使用情况调度器已启动，首次执行异步进行中，下次将在 %v 后执行", initialDelay)

	timer := time.NewTimer(initialDelay)
	defer timer.Stop()

	for {
		select {
		case <-d.ctx.Done():
			logger.Info("磁盘使用情况调度器已停止")
			return
		case <-timer.C:
			// 执行磁盘使用情况计算
			d.executeTask()

			// 重置定时器为 24 小时
			timer.Reset(24 * time.Hour)
		}
	}
}

// executeTask 执行计算任务（带互斥保护）
func (d *DiskUsageScheduler) executeTask() {
	// 尝试获取锁，如果获取失败说明已有任务在执行，直接返回
	if !d.taskMu.TryLock() {
		logger.Info("磁盘使用情况计算任务正在执行中，跳过本次执行")
		return
	}
	defer d.taskMu.Unlock()

	logger.Info("开始执行磁盘使用情况计算任务")

	if d.fileSvc == nil {
		logger.Error("FileService 未初始化，无法执行磁盘使用情况计算")
		return
	}

	err := d.fileSvc.CalculateAllUsersDiskUsage()
	if err != nil {
		logger.Error("执行磁盘使用情况计算失败", zap.Error(err))
		return
	}

	logger.Info("磁盘使用情况计算任务执行完成")
}

// Stop 停止调度器
func (d *DiskUsageScheduler) Stop() {
	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = false
	d.mu.Unlock()

	// 取消上下文，停止 Run() 循环中的 Timer
	d.cancel()

	// 等待所有 goroutine 退出
	d.wg.Wait()

	logger.Info("磁盘使用情况调度器已停止")
}
