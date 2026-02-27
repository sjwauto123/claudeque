package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	"cloudque/pkg/websocket"
	"context"
	"encoding/json"
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
	Done     chan struct{}
	Once     sync.Once
	mutex    sync.RWMutex
}

type systemInfoService struct {
	Pool      *websocket.ConnectionPool
	Collector *ResourceCollector
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
		SessionType: "ws",
		Role:        "admin", // 系统信息连接默认为管理员
		CreatedAt:   time.Now().Unix(),
	}

	// 使用连接池添加新客户端
	s.Pool.Add(userID, conn, metadata)

	// 资源收集器会定期收集并分发给所有管理员客户端
	logger.Infof("新的管理员websocket连接已建立，用户ID: %d", userID)
}

// NewResourceCollector 创建资源收集器
func NewResourceCollector(pool *websocket.ConnectionPool, procRepo repository.ProcessCacheRepository) *ResourceCollector {
	return &ResourceCollector{
		Pool:     pool,
		ProcRepo: procRepo,
		Done:     make(chan struct{}),
	}
}

// Start 启动资源收集
func (rc *ResourceCollector) Start() {
	rc.Once.Do(func() {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-rc.Done:
					return
				case <-ticker.C:
					// 检查连接数
					connectionCount := rc.Pool.GetAdminConnectionCount()
					// 根据连接数决定是否收集信息
					if connectionCount > 0 {
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
						rc.Pool.BroadcastToAdmins(data)
					}
				}
			}
		}()
	})
}

// collectSystemInfo 收集系统信息
func (rc *ResourceCollector) collectSystemInfo() *response.SystemInfosResponse {
	diskInfo, err := getDiskInfo()
	if err != nil {
		logger.Errorf("Failed to get disk info: %v", err)
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
	info.CpuList = append(info.CpuList, *diskInfo)
	info.CpuList = append(info.CpuList, *memoryInfo)
	info.GpuList = append(info.GpuList, gpuInfo...)
	info.ProcessList = append(info.ProcessList, processInfos...)
	return &info
}

// 获取磁盘信息（以根分区为例）
func getDiskInfo() (*response.CpuInfoResponse, error) {
	mountPoint := "/"
	//if runtime.GOOS == "windows" {
	//	mountPoint = "C:\\"
	//}

	usage, err := disk.Usage(mountPoint)
	if err != nil {
		return nil, err
	}

	return &response.CpuInfoResponse{
		DeviceName: "磁盘",
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
		if _, ok := err.(*exec.Error); ok {
			return []response.GPUInfoResponse{}, nil // 无 GPU 设备
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
	// 创建带超时的上下文，防止Redis操作超时
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	//从redis获取进程信息
	_, processes, err := rc.ProcRepo.GetAllPid(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询进程信息失败: %w", err)
	}
	var processInfos []response.ProcessInfoResponse
	for _, p := range processes {
		// 通过本地命令获取进程的详细信息
		info, err := getProcessDetails(p)
		if err != nil {
			logger.Infof("获取进程详细信息失败:%v", err)
		}
		processInfos = append(processInfos, info)
	}

	return processInfos, nil
}

// getProcessDetails 获取进程的详细信息
func getProcessDetails(pid int) (response.ProcessInfoResponse, error) {
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
		logger.Infof("无法获取进程命令行: %v", err)
	}

	// 计算运行时间
	startTime := time.Unix(createTime/1000, 0).Format("2006-01-02 15:04:05")
	duration := time.Since(time.Unix(createTime/1000, 0))
	r := formatDuration(duration)

	// 获取GPU信息
	gpuName := getGPUNameByPID(pid)

	return response.ProcessInfoResponse{
		Username:  username,
		PID:       strconv.Itoa(pid),
		GPUname:   gpuName,
		StartTime: startTime,
		IsNormal:  1,
		Runtime:   r,
		Command:   cmdline,
	}, nil
}

// getGPUNameByPID 根据进程ID获取GPU名称
func getGPUNameByPID(pid int) string {
	// 使用nvidia-smi命令获取进程所在的GPU信息
	cmd := exec.Command("nvidia-smi", "--query-compute-apps=pid,gpu_name", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), ", ")
		if len(parts) == 2 {
			pidStr := parts[0]
			if pidStr == strconv.Itoa(pid) {
				return parts[1]
			}
		}
	}

	return ""
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
