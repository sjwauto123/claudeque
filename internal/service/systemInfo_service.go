package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/throttle"
	"cloudque/pkg/websocket"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/process"
)

// Redis 键名常量
const (
	RedisKeyRetainedPIDs = "gpu_task:retained_pids"
	RedisKeyConfig       = "gpu_task:config"
)

// 默认配置值
const (
	DefaultAutoTerminateEnabled = false
	DefaultMaxDurationMinutes   = 3
)

// ResourceCollector 资源收集器
type ResourceCollector struct {
	Pool         *websocket.ConnectionPool
	Repo         repository.SystemInfoRepository
	Redis        repository.RedisRepository
	ProcessCache repository.ProcessCacheRepository
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	Once         sync.Once

	collectThrottle  *throttle.ErrorThrottle
	autoTermThrottle *throttle.ErrorThrottle
	cleanupThrottle  *throttle.ErrorThrottle
}

type systemInfoService struct {
	Pool      *websocket.ConnectionPool
	collector *ResourceCollector
	Redis     repository.RedisRepository
}

// Stop 停止系统信息服务
func (s *systemInfoService) Stop() {
	s.collector.Stop()
	logger.Info("系统信息服务已停止")
}

// NewSystemInfoService 创建系统信息服务
func NewSystemInfoService(pool *websocket.ConnectionPool, repo repository.SystemInfoRepository, redisRepo repository.RedisRepository, processCache repository.ProcessCacheRepository) SystemInfoService {
	collector := NewResourceCollector(pool, repo, redisRepo, processCache)
	collector.Start()
	return &systemInfoService{
		Pool:      pool,
		collector: collector,
		Redis:     redisRepo,
	}
}

// HandleSyMessage 处理请求创建连接
func (s *systemInfoService) HandleSyMessage(conn *ws.Conn, userID int) {
	// 创建会话元数据，设置角色为管理员
	metadata := &websocket.SessionMetadata{
		UserID:      userID,
		SessionType: "systemInfo",
		Role:        "admin", // 系统信息连接默认为管理员
		CreatedAt:   time.Now().Unix(),
	}

	// 使用连接池添加新客户端
	s.Pool.Add(userID, conn, metadata, nil)

	// 资源收集器会定期收集并分发给所有管理员客户端
	logger.Infof("新的管理员websocket连接已建立，进行接收系统消息，用户ID: %d", userID)
}

// ==================== 业务方法 ====================

// terminateProcess 终止进程（内部复用）
func terminateProcess(pid int) error {
	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// TerminateProcess 手动中断进程
func (s *systemInfoService) TerminateProcess(pid int) error {
	ctx := context.Background()

	// 检查 PID 是否存在于 GPU 进程中
	exists, err := s.isProcessInGPU(ctx, pid)
	if err != nil {
		return errors.New(errors.CodeInternalError, "检查进程状态失败")
	}
	if !exists {
		return errors.New(errors.CodeBadRequest, "进程不存在或不在 GPU 上运行")
	}

	if err := terminateProcess(pid); err != nil {
		logger.Errorf("终止进程 %d 失败：%v", pid, err)
		return errors.New(errors.CodeInternalError, "终止进程失败")
	}
	logger.Infof("手动终止进程：%d", pid)
	return nil
}

// RetainProcess 保留进程（不被自动中断）
func (s *systemInfoService) RetainProcess(pid int) error {
	ctx := context.Background()

	// 检查 PID 是否存在于 GPU 进程中
	exists, err := s.isProcessInGPU(ctx, pid)
	if err != nil {
		return errors.New(errors.CodeInternalError, "检查进程状态失败")
	}
	if !exists {
		return errors.New(errors.CodeBadRequest, "进程不存在或不在 GPU 上运行")
	}

	err = s.Redis.SAdd(ctx, RedisKeyRetainedPIDs, strconv.Itoa(pid))
	if err != nil {
		return errors.New(errors.CodeInternalError, "保留进程失败")
	}
	logger.Infof("保留进程：%d", pid)
	return nil
}

// CancelRetain 取消保留
func (s *systemInfoService) CancelRetain(pid int) error {
	ctx := context.Background()

	// 检查 PID 是否在保留列表中
	pidStr := strconv.Itoa(pid)
	exists, err := s.Redis.SIsMember(ctx, RedisKeyRetainedPIDs, pidStr)
	if err != nil {
		return errors.New(errors.CodeInternalError, "检查保留状态失败")
	}
	if !exists {
		return errors.New(errors.CodeBadRequest, "该进程未被保留")
	}

	err = s.Redis.SRem(ctx, RedisKeyRetainedPIDs, pidStr)
	if err != nil {
		return errors.New(errors.CodeInternalError, "取消保留失败")
	}
	logger.Infof("取消保留进程：%d", pid)
	return nil
}

// isProcessInGPU 检查 PID 是否存在于 GPU 进程中
func (s *systemInfoService) isProcessInGPU(ctx context.Context, pid int) (bool, error) {
	pids, err := s.collector.Repo.GetGPUPIDs(ctx)
	if err != nil {
		return false, err
	}
	return pids[strconv.Itoa(pid)], nil
}

// GetConfig 获取全局配置
func (s *systemInfoService) GetConfig() *response.ConfigResponse {
	ctx := context.Background()
	config, err := s.Redis.HGetAll(ctx, RedisKeyConfig)
	if err != nil {
		logger.Errorf("获取配置失败：%v", err)
		return &response.ConfigResponse{
			AutoTerminateEnabled: DefaultAutoTerminateEnabled,
			MaxDurationMinutes:   DefaultMaxDurationMinutes,
		}
	}

	enabled, err := strconv.ParseBool(config["auto_terminate_enabled"])
	if err != nil {
		logger.Errorf("解析自动中断配置失败：%v", err)
		enabled = DefaultAutoTerminateEnabled
	}
	duration, err := strconv.Atoi(config["max_duration_minutes"])
	if err != nil || duration == 0 {
		duration = DefaultMaxDurationMinutes
	}

	return &response.ConfigResponse{
		AutoTerminateEnabled: enabled,
		MaxDurationMinutes:   duration,
	}
}

// UpdateConfig 更新全局配置
func (s *systemInfoService) UpdateConfig(enabled *bool, duration *int) error {
	ctx := context.Background()

	// 只更新传了的字段
	fields := make(map[string]interface{})

	if enabled != nil {
		fields["auto_terminate_enabled"] = strconv.FormatBool(*enabled)
	}
	if duration != nil {
		fields["max_duration_minutes"] = strconv.Itoa(*duration)
	}

	if len(fields) == 0 {
		return errors.New(errors.CodeBadRequest, "至少需要提供一个配置项")
	}

	err := s.Redis.HSet(ctx, RedisKeyConfig, fields)
	if err != nil {
		return errors.New(errors.CodeInternalError, "更新配置失败")
	}
	logger.Infof("更新配置：%v", fields)
	return nil
}

// ==================== 资源收集器 ====================

// NewResourceCollector 创建资源收集器
func NewResourceCollector(pool *websocket.ConnectionPool, repo repository.SystemInfoRepository, redisRepo repository.RedisRepository, processCache repository.ProcessCacheRepository) *ResourceCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &ResourceCollector{
		Pool:             pool,
		Repo:             repo,
		Redis:            redisRepo,
		ProcessCache:     processCache,
		ctx:              ctx,
		cancel:           cancel,
		collectThrottle:  throttle.NewErrorThrottle(time.Minute),
		autoTermThrottle: throttle.NewErrorThrottle(time.Minute),
		cleanupThrottle:  throttle.NewErrorThrottle(5 * time.Minute),
	}
}

// Start 启动资源收集
func (rc *ResourceCollector) Start() {
	rc.Once.Do(func() {
		// 1. 系统信息推送（每 2 秒）
		rc.wg.Add(1)
		go rc.startInfoPusher()

		// 2. 自动中断检查（每 60 秒）
		rc.wg.Add(1)
		go rc.startAutoTerminator()

		// 3. 保留任务清理（每 5 分钟）
		rc.wg.Add(1)
		go rc.startRetainedTaskCleaner()

		logger.Info("资源收集器已启动")
	})
}

// startInfoPusher 系统信息推送
func (rc *ResourceCollector) startInfoPusher() {
	defer rc.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-rc.ctx.Done():
			logger.Info("信息推送器已停止")
			return
		case <-ticker.C:
			if rc.Pool.IsHavingSystemInfoConnection() {
				info := rc.collectSystemInfo()
				if info == nil {
					rc.collectThrottle.Log("系统信息收集失败")
					continue
				}
				data, err := json.Marshal(info)
				if err != nil {
					rc.collectThrottle.Log("JSON转换失败: %v", err)
					continue
				}
				rc.Pool.BroadcastToAdminsByType("systemInfo", data)
			}
		}
	}
}

// ==================== 定时任务 ====================

// startAutoTerminator 自动中断检查
func (rc *ResourceCollector) startAutoTerminator() {
	defer rc.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-rc.ctx.Done():
			logger.Info("自动中断检查器已停止")
			return
		case <-ticker.C:
			rc.checkAndTerminate()
		}
	}
}

// checkAndTerminate 检查并终止超时进程
func (rc *ResourceCollector) checkAndTerminate() {
	ctx := context.Background()

	// 获取配置（键不存在时使用默认值）
	config, err := rc.Redis.HGetAll(ctx, RedisKeyConfig)
	if err != nil {
		rc.autoTermThrottle.Log("获取配置失败：%v", err)
		return
	}

	// 解析自动中断开关，默认启用
	enabled := DefaultAutoTerminateEnabled
	if val, ok := config["auto_terminate_enabled"]; ok && val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enabled = parsed
		} else {
			rc.autoTermThrottle.Log("解析自动中断配置失败：%v", err)
		}
	}
	if !enabled {
		return
	}

	// 解析最大时长，默认 3 分钟
	maxDuration := DefaultMaxDurationMinutes
	if val, ok := config["max_duration_minutes"]; ok && val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			maxDuration = parsed
		} else {
			rc.autoTermThrottle.Log("解析最大时长配置失败：%v", err)
		}
	}
	maxDurationSeconds := maxDuration * 60

	// 获取所有 GPU 进程的 PID 和运行时长
	processDurations, err := rc.Repo.GetProcessDurations(ctx)
	if err != nil {
		rc.autoTermThrottle.Log("获取进程运行时长失败：%v", err)
		return
	}

	// 获取系统任务 PID（排除）
	_, systemPIDs, err := rc.ProcessCache.GetAllPid(ctx)
	if err != nil {
		rc.autoTermThrottle.Log("获取系统任务 PID 失败：%v", err)
		systemPIDs = []int{}
	}
	systemPIDSet := make(map[string]bool)
	for _, pid := range systemPIDs {
		systemPIDSet[strconv.Itoa(pid)] = true
	}

	// 获取保留的 PID（排除）
	retainedPIDs, err := rc.Redis.SMembers(ctx, RedisKeyRetainedPIDs)
	if err != nil {
		logger.Errorf("获取保留 PID 失败：%v", err)
		retainedPIDs = []string{}
	}
	retainedPIDSet := make(map[string]bool)
	for _, pid := range retainedPIDs {
		retainedPIDSet[pid] = true
	}

	// 检查服务器任务
	for _, proc := range processDurations {
		// 排除系统任务和保留任务
		if systemPIDSet[proc.PID] || retainedPIDSet[proc.PID] {
			continue
		}

		// 超时则 kill
		if proc.RunningDurationSecs > maxDurationSeconds {
			pid, err := strconv.Atoi(proc.PID)
			if err != nil {
				rc.autoTermThrottle.Log("解析 PID 失败：%s", proc.PID)
				continue
			}
			err = terminateProcess(pid)
			if err != nil {
				rc.autoTermThrottle.Log("终止进程%v失败：%s", pid, err)
				continue
			}
			logger.Infof("自动终止超时进程：%s，运行时长：%d秒", proc.PID, proc.RunningDurationSecs)
		}
	}
}

// startRetainedTaskCleaner 保留任务清理
func (rc *ResourceCollector) startRetainedTaskCleaner() {
	defer rc.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rc.ctx.Done():
			logger.Info("保留任务清理器已停止")
			return
		case <-ticker.C:
			rc.cleanupRetainedTasks()
		}
	}
}

// cleanupRetainedTasks 清理已完成的保留任务
func (rc *ResourceCollector) cleanupRetainedTasks() {
	ctx := context.Background()
	retainedPIDs, err := rc.Redis.SMembers(ctx, RedisKeyRetainedPIDs)
	if err != nil {
		rc.cleanupThrottle.Log("获取保留 PID 失败：%v", err)
		return
	}

	for _, pidStr := range retainedPIDs {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			rc.cleanupThrottle.Log("解析 PID 失败：%s", pidStr)
			continue
		}
		if !isProcessRunning(pid) {
			err := rc.Redis.SRem(ctx, RedisKeyRetainedPIDs, pidStr)
			if err != nil {
				rc.cleanupThrottle.Log("移除保留 PID 失败：%s，错误：%v", pidStr, err)
				continue
			}
			logger.Infof("清理已完成的保留任务：%s", pidStr)
		}
	}
}

// Stop 停止资源收集
func (rc *ResourceCollector) Stop() {
	// 取消上下文，通知协程退出
	rc.cancel()
	// 等待协程退出
	rc.wg.Wait()
	logger.Info("资源收集器已完全停止")
}

// ==================== 数据收集 ====================

// collectSystemInfo 收集系统信息
func (rc *ResourceCollector) collectSystemInfo() *response.SystemInfosResponse {
	ctx, cancel := context.WithTimeout(rc.ctx, 2*time.Second)
	defer cancel()

	var info response.SystemInfosResponse

	diskHome, err := rc.Repo.GetDiskInfo("/home")
	if err != nil {
		rc.collectThrottle.Log("Failed to get /home info: %v", err)
	} else if diskHome != nil {
		info.CpuList = append(info.CpuList, *diskHome)
	}

	diskMain, err := rc.Repo.GetDiskInfo("/")
	if err != nil {
		rc.collectThrottle.Log("Failed to get / info: %v", err)
	} else if diskMain != nil {
		info.CpuList = append(info.CpuList, *diskMain)
	}

	memoryInfo, err := rc.Repo.GetMemoryInfo()
	if err != nil {
		rc.collectThrottle.Log("Failed to get memory info: %v", err)
	} else if memoryInfo != nil {
		info.CpuList = append(info.CpuList, *memoryInfo)
	}

	gpuInfo, err := rc.Repo.GetGPUInfo(ctx)
	if err != nil {
		rc.collectThrottle.Log("Failed to get GPU info: %v", err)
	} else {
		info.GpuList = append(info.GpuList, gpuInfo...)
	}

	// 收集并分类进程信息
	systemProcesses, serverProcesses := rc.collectAndClassifyProcesses(ctx)
	info.SystemProcesses = systemProcesses
	info.ServerProcesses = serverProcesses

	return &info
}

// collectAndClassifyProcesses 收集并分类进程信息
func (rc *ResourceCollector) collectAndClassifyProcesses(ctx context.Context) ([]response.SystemProcessInfo, []response.ServerProcessInfo) {
	// 获取所有进程详细信息
	processInfos, err := rc.Repo.GetProcessInfo(ctx)
	if err != nil {
		rc.collectThrottle.Log("获取进程信息失败：%v", err)
		return []response.SystemProcessInfo{}, []response.ServerProcessInfo{}
	}

	_, systemPIDs, err := rc.ProcessCache.GetAllPid(ctx)
	if err != nil {
		rc.collectThrottle.Log("获取系统任务 PID 失败：%v", err)
		systemPIDs = []int{}
	}
	systemPIDSet := make(map[string]bool)
	for _, pid := range systemPIDs {
		systemPIDSet[strconv.Itoa(pid)] = true
	}

	// 获取保留的 PID 集合
	retainedPIDs, err := rc.Redis.SMembers(ctx, RedisKeyRetainedPIDs)
	if err != nil {
		rc.collectThrottle.Log("获取保留 PID 失败：%v", err)
		retainedPIDs = []string{}
	}
	retainedPIDSet := make(map[string]bool)
	for _, pid := range retainedPIDs {
		retainedPIDSet[pid] = true
	}

	var systemProcesses []response.SystemProcessInfo
	var serverProcesses []response.ServerProcessInfo

	for _, proc := range processInfos {
		if systemPIDSet[proc.PID] {
			// 系统任务
			systemProcesses = append(systemProcesses, response.SystemProcessInfo{
				Username:  proc.Username,
				PID:       proc.PID,
				JobName:   proc.JobName,
				GPUname:   proc.GPUname,
				StartTime: proc.StartTime,
				IsNormal:  proc.IsNormal,
				Runtime:   proc.Runtime,
				Command:   proc.Command,
			})
		} else {
			// 服务器任务
			isRetained := retainedPIDSet[proc.PID]

			serverProcesses = append(serverProcesses, response.ServerProcessInfo{
				Username:   proc.Username,
				PID:        proc.PID,
				JobName:    proc.JobName,
				GPUname:    proc.GPUname,
				StartTime:  proc.StartTime,
				IsNormal:   proc.IsNormal,
				Runtime:    proc.Runtime,
				Command:    proc.Command,
				IsRetained: isRetained,
			})
		}
	}

	return systemProcesses, serverProcesses
}

// ==================== 辅助函数 ====================

// isProcessRunning 检查进程是否运行
func isProcessRunning(pid int) bool {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	running, err := p.IsRunning()
	if err != nil {
		return false
	}
	return running
}
