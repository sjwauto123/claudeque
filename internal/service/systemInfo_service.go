package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	"cloudque/pkg/websocket"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// ResourceCollector 资源收集器
type ResourceCollector struct {
	Pool     *websocket.ConnectionPool
	ProcRepo repository.ProcessCacheRepository
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	Once     sync.Once
	mutex    sync.RWMutex
}

type systemInfoService struct {
	Pool      *websocket.ConnectionPool
	Collector *ResourceCollector
}

// Stop 停止系统信息服务
func (s *systemInfoService) Stop() {
	s.Collector.Stop()
	logger.Info("系统信息服务已停止")
}

func NewSystemInfoService(pool *websocket.ConnectionPool, procRepo repository.ProcessCacheRepository) SystemInfoService {
	collector := NewResourceCollector(pool, procRepo)
	collector.Start()
	return &systemInfoService{
		Pool:      pool,
		Collector: collector,
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

// NewResourceCollector 创建资源收集器
func NewResourceCollector(pool *websocket.ConnectionPool, procRepo repository.ProcessCacheRepository) *ResourceCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &ResourceCollector{
		Pool:     pool,
		ProcRepo: procRepo,
		ctx:      ctx,
		cancel:   cancel,
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

// Start 启动资源收集
func (rc *ResourceCollector) Start() {
	rc.Once.Do(func() {
		rc.wg.Add(1)
		go func() {
			defer rc.wg.Done()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-rc.ctx.Done():
					logger.Info("资源收集器已停止")
					return
				case <-ticker.C:
					// 检查是否有ws连接连接
					connectionCount := rc.Pool.IsHavingSystemInfoConnection()
					if connectionCount {
						// 收集系统信息
						info := rc.collectSystemInfo()
						if info == nil {
							logger.Error("系统信息收集失败")
							continue
						}
						// 转换为JSON
						data, err := json.Marshal(info)
						if err != nil {
							logger.Errorf("JSON转换失败: %v", err)
							continue
						}
						// 分发给所有管理员客户端
						rc.Pool.BroadcastToAdminsByType("systemInfo", data)
					}
				}
			}
		}()
		logger.Info("资源收集器已启动")
	})
}

// collectSystemInfo 收集系统信息
func (rc *ResourceCollector) collectSystemInfo() *response.SystemInfosResponse {
	diskHome, err := getDiskInfo("/home")
	if err != nil {
		logger.Errorf("Failed to get /home info: %v", err)
	}

	diskMain, err := getDiskInfo("/")
	if err != nil {
		logger.Errorf("Failed to get / info: %v", err)
	}

	memoryInfo, err := getMemoryInfo()
	if err != nil {
		logger.Errorf("Failed to get memory info: %v", err)
	}

	gpuInfo, err := GetNvidiaGPUInfo()
	if err != nil {
		logger.Errorf("Failed to get GPU info: %v", err)
	}

	// 收集进程信息
	processInfos, err := rc.collectProcessInfo()
	if err != nil {
		logger.Errorf("Failed to get process info: %v", err)
	}

	var info response.SystemInfosResponse
	info.CpuList = append(info.CpuList, *diskMain, *diskHome, *memoryInfo)
	info.GpuList = append(info.GpuList, gpuInfo...)
	info.ProcessList = append(info.ProcessList, processInfos...)
	return &info
}

// 获取磁盘信息
func getDiskInfo(mountPoint string) (*response.CpuInfoResponse, error) {
	usage, err := disk.Usage(mountPoint)
	if err != nil {
		return nil, err
	}

	return &response.CpuInfoResponse{
		DeviceName: mountPoint + "分区",
		TotalCap:   bytesToGB(usage.Total),
		UseCap:     bytesToGB(usage.Used),
		RemainCap:  bytesToGB(usage.Free),
		Percentage: fmt.Sprintf("%.1f%%", usage.UsedPercent),
	}, nil
}

// 获取内存信息
func getMemoryInfo() (*response.CpuInfoResponse, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	total := vmStat.Total
	used := total - vmStat.Available // 已用 = 总量 - 可用
	free := vmStat.Available

	return &response.CpuInfoResponse{
		DeviceName: "内存",
		TotalCap:   bytesToGB(total),
		UseCap:     bytesToGB(used),
		RemainCap:  bytesToGB(free),
		Percentage: fmt.Sprintf("%.1f%%", vmStat.UsedPercent),
	}, nil
}

// bytesToGB: 转换字节为 GB 字符串（保留1位小数）
func bytesToGB(bytes uint64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1fGB", gb)
}

// GetNvidiaGPUInfo 获取显卡信息
func GetNvidiaGPUInfo() ([]response.GPUInfoResponse, error) {
	// 执行 nvidia-smi 命令，输出 CSV 格式
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		// 如果命令不存在（无 NVIDIA 驱动），返回空列表
		var err1 *exec.Error
		if errors.As(err, &err1) {
			return []response.GPUInfoResponse{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var gpus []response.GPUInfoResponse

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 6 {
			continue
		}

		index, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		name := strings.TrimSpace(fields[1])
		temp := strings.TrimSpace(fields[2]) + "°C"
		util := strings.TrimSpace(fields[3]) + "%"
		memUsed := strings.TrimSpace(fields[4]) + "MB"
		memTotal := strings.TrimSpace(fields[5]) + "MB"

		gpus = append(gpus, response.GPUInfoResponse{
			Index:      index,
			DeviceName: name,
			Temp:       temp,
			Util:       util,
			MemUsed:    memUsed,
			MemTotal:   memTotal,
		})
	}

	return gpus, nil
}

// collectProcessInfo 收集进程信息
func (rc *ResourceCollector) collectProcessInfo() ([]response.ProcessInfoResponse, error) {
	//创建带超时的上下文，防止Redis操作超时
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	//从redis获取进程信息
	jobNames, processes, err := rc.ProcRepo.GetAllPid(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询进程信息失败: %w", err)
	}

	if len(jobNames) == 0 || len(processes) == 0 || len(jobNames) != len(processes) {
		return []response.ProcessInfoResponse{}, nil
	}
	// 获取 Redis 中进程与 GPU 的映射
	gpuMap := getGPUMap(processes)
	if len(gpuMap) == 0 {
		return []response.ProcessInfoResponse{}, nil
	}

	var processInfos []response.ProcessInfoResponse
	for i, p := range processes {
		// 通过本地命令获取进程的详细信息
		info, err := getProcessDetails(p, gpuMap, jobNames[i])
		if err != nil {
			logger.Infof("获取进程 id 为%v 详细信息失败:%v", p, err)
		}
		processInfos = append(processInfos, info)
	}
	return processInfos, nil
}

// getGPUMap 获取指定进程与 GPU 的映射
func getGPUMap(processes []int) map[string]string {
	gpuMap := make(map[string]string)

	// 使用 nvidia-smi 命令获取所有进程所在的 GPU 信息
	cmd := exec.Command("nvidia-smi", "--query-compute-apps=pid,gpu_name", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return gpuMap
	}

	// 构建需要查询的 PID 集合，用于快速查找
	pidSet := make(map[string]bool)
	for _, pid := range processes {
		pidSet[strconv.Itoa(pid)] = true
	}

	// 只保留我们关心的进程
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), ", ")
		if len(parts) == 2 {
			pidStr := parts[0]
			// 只保留 Redis 中存在的进程
			if pidSet[pidStr] {
				gpuName := parts[1]
				gpuMap[pidStr] = gpuName
			}
		}
	}

	return gpuMap
}

// getProcessDetails 获取进程的详细信息
func getProcessDetails(pid int, gpuMap map[string]string, jobName string) (response.ProcessInfoResponse, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return response.ProcessInfoResponse{}, err
	}

	// 获取进程信息
	username, err := p.Username()
	if err != nil {
		logger.Infof("无法获取进程用户名: %v", err)
	}
	createTime, err := p.CreateTime()
	if err != nil {
		logger.Infof("无法获取进程创建时间: %v", err)
	}
	cmdline, err := p.Cmdline()
	if err != nil {
		logger.Infof("无法获取进程命令行：%v", err)
	}

	// 计算运行时间
	startTime := time.Unix(createTime/1000, 0).Format("2006-01-02 15:04:05")
	duration := time.Since(time.Unix(createTime/1000, 0))
	r := formatDuration(duration)

	// 从映射中获取 GPU 信息
	gpuName := gpuMap[strconv.Itoa(pid)]
	if gpuName == "" {
		gpuName = "未获取到显卡信息"
	}

	isNormal := 1
	// 如果进程已经退出，IsRunning() 会返回 false
	if running, err := p.IsRunning(); err != nil || !running {
		isNormal = 0
	}

	return response.ProcessInfoResponse{
		Username:  username,
		PID:       strconv.Itoa(pid),
		JobName:   jobName,
		GPUname:   gpuName,
		StartTime: startTime,
		IsNormal:  isNormal,
		Runtime:   r,
		Command:   cmdline,
	}, nil
}

// formatDuration 格式化时间间隔，只保留整数秒部分
func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	var result string
	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 || hours > 0 {
		result += fmt.Sprintf("%dm", minutes)
	}
	result += fmt.Sprintf("%ds", secs)

	return result
}
