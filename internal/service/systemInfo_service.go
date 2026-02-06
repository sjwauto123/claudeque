package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	"cloudque/pkg/ws"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// ResourceCollector 资源收集器
type ResourceCollector struct {
	Hub           *ws.Hub
	ProcRepo      repository.ProcessRepository
	Done          chan struct{}
	Once          sync.Once
	processCache  []entity.Process
	cacheExpiry   time.Time
	cacheDuration time.Duration
	isRunning     bool
	mutex         sync.RWMutex
}

// NewResourceCollector 创建资源收集器
func NewResourceCollector(hub *ws.Hub, procRepo repository.ProcessRepository) *ResourceCollector {
	return &ResourceCollector{
		Hub:           hub,
		ProcRepo:      procRepo,
		Done:          make(chan struct{}),
		cacheDuration: 20 * time.Second, // 缓存10秒
		isRunning:     false,
	}
}

// Start 启动资源收集
func (rc *ResourceCollector) Start() {
	rc.Once.Do(func() {
		go func() {
			for {
				select {
				case <-rc.Done:
					return
				case <-rc.Hub.StartCollect:
					rc.startCollection()
				case <-rc.Hub.StopCollect:
					rc.stopCollection()
				}
			}
		}()
	})
}

// startCollection 开始收集
func (rc *ResourceCollector) startCollection() {
	rc.mutex.Lock()
	if rc.isRunning {
		rc.mutex.Unlock()
		return
	}
	rc.isRunning = true
	rc.mutex.Unlock()

	logger.Info("开始系统信息收集")

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			rc.mutex.RLock()
			running := rc.isRunning
			rc.mutex.RUnlock()

			if !running {
				return
			}

			select {
			case <-rc.Done:
				return
			case <-ticker.C:
				// 收集系统信息
				info := rc.collectSystemInfo()
				fmt.Println(info)
				// 转换为JSON
				data := JsonToByte(info)
				// 分发给所有管理员
				rc.Hub.SendToAllAdmins(data)
			}
		}
	}()
}

// stopCollection 停止收集
func (rc *ResourceCollector) stopCollection() {
	rc.mutex.Lock()
	if !rc.isRunning {
		rc.mutex.Unlock()
		return
	}
	rc.isRunning = false
	rc.mutex.Unlock()

	logger.Info("停止系统信息收集")
}

// Stop 停止资源收集
func (rc *ResourceCollector) Stop() {
	rc.Once.Do(func() {
		close(rc.Done)
	})
}

// collectSystemInfo 收集系统信息
func (rc *ResourceCollector) collectSystemInfo() *response.SystemMessages {
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

	var info response.SystemMessages
	info.CpuList = append(info.CpuList, *diskInfo)
	info.CpuList = append(info.CpuList, *memoryInfo)
	info.GpuList = append(info.GpuList, gpuInfo...)
	info.ProcessList = append(info.ProcessList, processInfos...)
	return &info
}

// collectProcessInfo 收集进程信息
func (rc *ResourceCollector) collectProcessInfo() ([]response.ProcessInfo, error) {
	// 1. 从缓存或数据库获取进程信息
	processes, err := rc.getProcessesFromCache()
	fmt.Println("获取的进程：", processes)
	if err != nil {
		return nil, fmt.Errorf("查询进程信息失败: %w", err)
	}

	var processInfos []response.ProcessInfo

	// 2. 检查每个进程是否真的在运行
	for _, p := range processes {
		// 检查进程是否存在
		if isProcessRunning(p.PID) {

			// 3. 通过本地命令获取进程的详细信息
			info, err := getProcessDetails(p.PID)
			fmt.Println("获取的进程详情：", info)
			if err == nil {
				processInfos = append(processInfos, info)
			}
		}
	}

	return processInfos, nil
}

// getProcessesFromCache 从缓存或数据库获取进程信息
func (rc *ResourceCollector) getProcessesFromCache() ([]entity.Process, error) {
	rc.mutex.RLock()
	cacheValid := time.Now().Before(rc.cacheExpiry)
	cachedProcesses := rc.processCache
	rc.mutex.RUnlock()

	if cacheValid && len(cachedProcesses) > 0 {
		logger.Info("使用缓存的进程信息")
		return cachedProcesses, nil
	}

	// 缓存过期，从数据库查询
	logger.Info("从数据库查询进程信息")
	processes, err := rc.ProcRepo.FindAll()
	if err != nil {
		return nil, err
	}

	// 更新缓存
	rc.mutex.Lock()
	rc.processCache = processes
	rc.cacheExpiry = time.Now().Add(rc.cacheDuration)
	rc.mutex.Unlock()
	return processes, nil
}

type systemInfoService struct {
	Hub       *ws.Hub
	Collector *ResourceCollector
}

func NewSystemInfoService(hub *ws.Hub, procRepo repository.ProcessRepository) SystemInfoService {
	collector := NewResourceCollector(hub, procRepo)
	collector.Start()
	return &systemInfoService{
		Hub:       hub,
		Collector: collector,
	}
}

func (s *systemInfoService) HandleSyMessage(conn *websocket.Conn) {
	// 这里假设所有连接都是管理员
	client := ws.NewClient(s.Hub, conn, "admin")
	s.Hub.Register <- client

	// 标记为管理员客户端
	s.Hub.AdminClients[client] = true

	//// 开启读协程
	//go client.ReadPump()
	// 开启写协程
	go client.WritePump()

	// 注意：现在不需要为每个客户端单独启动收集协程
	// 资源收集器会定期收集并分发给所有管理员客户端
}

// GetSystemInfo 仅负责采集数据，不涉及推送！
func (s *systemInfoService) GetSystemInfo() *response.SystemMessages {
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

	var info response.SystemMessages
	info.CpuList = append(info.CpuList, *diskInfo)
	info.CpuList = append(info.CpuList, *memoryInfo)
	info.GpuList = append(info.GpuList, gpuInfo...)
	return &info
}

func GetNvidiaGPUInfo() ([]response.GPUInfo, error) {
	// 执行 nvidia-smi 命令，输出 CSV 格式
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		// 如果命令不存在（无 NVIDIA 驱动），返回空列表
		if _, ok := err.(*exec.Error); ok {
			return []response.GPUInfo{}, nil // 无 GPU 设备
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var gpus []response.GPUInfo

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

		gpus = append(gpus, response.GPUInfo{
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

// bytesToGB: 转换字节为 GB 字符串（保留1位小数）
func bytesToGB(bytes uint64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1f", gb)
}

// 获取磁盘信息（以根分区为例）
func getDiskInfo() (*response.SystemInfo, error) {
	// 获取主挂载点（Linux/macOS 用 "/", Windows 用 "C:\\")
	mountPoint := "/"
	if runtime.GOOS == "windows" {
		mountPoint = "C:\\"
	}

	usage, err := disk.Usage(mountPoint)
	if err != nil {
		return nil, err
	}

	return &response.SystemInfo{
		DeviceName: "disk", // 👈 关键：标识为磁盘
		TotalCap:   bytesToGB(usage.Total),
		UseCap:     bytesToGB(usage.Used),
		RemainCap:  bytesToGB(usage.Free),
		Percentage: fmt.Sprintf("%.1f%%", usage.UsedPercent),
	}, nil
}

// 获取内存信息
func getMemoryInfo() (*response.SystemInfo, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	total := vmStat.Total
	used := total - vmStat.Available // 已用 = 总量 - 可用
	free := vmStat.Available

	return &response.SystemInfo{
		DeviceName: "memory", // 👈 关键：标识为内存
		TotalCap:   bytesToGB(total),
		UseCap:     bytesToGB(used),
		RemainCap:  bytesToGB(free),
		Percentage: fmt.Sprintf("%.1f%%", vmStat.UsedPercent),
	}, nil
}
func JsonToByte(msg *response.SystemMessages) []byte {
	marshal, err := json.Marshal(msg)
	if err != nil {
		log.Println(err)
	}
	return marshal
}

// isProcessRunning 检查进程是否正在运行
func isProcessRunning(pid int) bool {
	// 跨平台检查进程是否存在
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}

	// 尝试不同的方法检查进程状态
	// 在Windows上，Status()方法可能不可用，尝试使用其他方法
	if runtime.GOOS == "windows" {
		// 在Windows上，尝试获取进程名称
		_, err = p.Name()
		return err == nil
	} else {
		// 在Linux上，使用Status()方法
		_, err = p.Status()
		return err == nil
	}
}

// isProcessOnGPU 检查进程是否在指定的显卡上运行
func isProcessOnGPU(pid int, gpuID int) bool {
	// 使用nvidia-smi命令检查进程是否在指定显卡上
	// 在Windows上，需要确保nvidia-smi在PATH中
	cmd := exec.Command("nvidia-smi", "--query-compute-apps=pid,gpu_uuid", "--format=csv,noheader")

	// 在Windows上，可能需要指定完整路径
	if runtime.GOOS == "windows" {
		// 尝试常见的nvidia-smi路径
		nvidiaPaths := []string{
			"C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
			"C:\\Program Files (x86)\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
		}

		// 检查是否存在nvidia-smi.exe
		for _, path := range nvidiaPaths {
			if _, err := os.Stat(path); err == nil {
				cmd = exec.Command(path, "--query-compute-apps=pid,gpu_uuid", "--format=csv,noheader")
				break
			}
		}
	}

	output, err := cmd.Output()
	if err != nil {
		// 如果命令执行失败，尝试直接返回true（假设进程在GPU上运行）
		// 因为在开发环境中，nvidia-smi可能不可用
		return true
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), ", ")
		if len(parts) == 2 {
			pidStr := parts[0]
			if pidStr == strconv.Itoa(pid) {
				// 这里简化处理，实际需要根据gpu_uuid映射到gpuID
				return true
			}
		}
	}

	return false
}

// getProcessDetails 获取进程的详细信息
func getProcessDetails(pid int) (response.ProcessInfo, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return response.ProcessInfo{}, err
	}

	// 获取进程信息
	username, _ := p.Username()
	createTime, _ := p.CreateTime()
	cmdline, _ := p.Cmdline()

	// 计算运行时间
	startTime := time.Unix(createTime/1000, 0).Format("2006-01-02 15:04:05")
	runtime := time.Since(time.Unix(createTime/1000, 0)).String()

	// 获取GPU信息
	gpuName := getGPUNameByPID(pid)

	return response.ProcessInfo{
		Username:  username,
		PID:       strconv.Itoa(pid),
		GPUname:   gpuName,
		StartTime: startTime,
		IsNormal:  1, // 假设进程正常运行
		Runtime:   runtime,
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
