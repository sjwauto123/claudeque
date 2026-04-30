package repository

import (
	"cloudque/internal/model/dto/response"
	"cloudque/pkg/throttle"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type systemInfoRepository struct {
	gpuThrottle     *throttle.ErrorThrottle
	processThrottle *throttle.ErrorThrottle
}

// NewSystemInfoRepository 创建系统信息仓储实例
func NewSystemInfoRepository() SystemInfoRepository {
	return &systemInfoRepository{
		gpuThrottle:     throttle.NewErrorThrottle(time.Minute),
		processThrottle: throttle.NewErrorThrottle(time.Minute),
	}
}

// GetDiskInfo 获取磁盘信息
func (r *systemInfoRepository) GetDiskInfo(mountPoint string) (*response.CpuInfoResponse, error) {
	usage, err := disk.Usage(mountPoint)
	if err != nil {
		return nil, fmt.Errorf("获取磁盘信息失败: %w", err)
	}

	return &response.CpuInfoResponse{
		DeviceName: mountPoint + "分区",
		TotalCap:   bytesToGB(usage.Total),
		UseCap:     bytesToGB(usage.Used),
		RemainCap:  bytesToGB(usage.Free),
		Percentage: fmt.Sprintf("%.1f%%", usage.UsedPercent),
	}, nil
}

// GetMemoryInfo 获取内存信息
func (r *systemInfoRepository) GetMemoryInfo() (*response.CpuInfoResponse, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("获取内存信息失败: %w", err)
	}

	total := vmStat.Total
	used := total - vmStat.Available
	free := vmStat.Available

	return &response.CpuInfoResponse{
		DeviceName: "内存",
		TotalCap:   bytesToGB(total),
		UseCap:     bytesToGB(used),
		RemainCap:  bytesToGB(free),
		Percentage: fmt.Sprintf("%.1f%%", vmStat.UsedPercent),
	}, nil
}

// GetGPUInfo 获取 GPU显卡 信息
func (r *systemInfoRepository) GetGPUInfo(ctx context.Context) ([]response.GPUInfoResponse, error) {

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return []response.GPUInfoResponse{}, nil
		}
		return nil, fmt.Errorf("执行 nvidia-smi 命令失败: %w", err)
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

		index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			r.gpuThrottle.Log("解析 GPU 索引失败: %v", err)
			continue
		}
		name := strings.TrimSpace(fields[1])
		temp := strings.TrimSpace(fields[2]) + "°C"
		util := strings.TrimSpace(fields[3]) + "%"
		memUsed := strings.TrimSpace(fields[4]) + "MB"
		memTotal := strings.TrimSpace(fields[5]) + "MB"

		gpus = append(gpus, response.GPUInfoResponse{
			Index:      index,
			DeviceName: fmt.Sprintf("%d-%s", index, name),
			Temp:       temp,
			Util:       util,
			MemUsed:    memUsed,
			MemTotal:   memTotal,
		})
	}

	return gpus, nil
}

// GetGPUPIDs 获取 GPU 上所有进程的 PID 集合
func (r *systemInfoRepository) GetGPUPIDs(ctx context.Context) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return make(map[string]bool), nil
		}
		return nil, fmt.Errorf("执行 nvidia-smi 命令失败: %w", err)
	}

	pids := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		pid := strings.TrimSpace(line)
		if pid != "" {
			pids[pid] = true
		}
	}
	return pids, nil
}

// GetProcessDurations 获取 GPU 上所有进程的 PID 和运行时长
func (r *systemInfoRepository) GetProcessDurations(ctx context.Context) ([]ProcessDuration, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return []ProcessDuration{}, nil
		}
		return nil, fmt.Errorf("执行 nvidia-smi 命令失败: %w", err)
	}

	var durations []ProcessDuration
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		pidStr := strings.TrimSpace(line)
		if pidStr == "" {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		p, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}

		createTime, err := p.CreateTime()
		if err != nil {
			continue
		}

		duration := time.Since(time.Unix(createTime/1000, 0))
		durations = append(durations, ProcessDuration{
			PID:                 pidStr,
			RunningDurationSecs: int(duration.Seconds()),
		})
	}
	return durations, nil
}

// GetProcessInfo 获取全部进程的详细信息
func (r *systemInfoRepository) GetProcessInfo(ctx context.Context) ([]ProcessInfo, error) {
	processList, err := r.getProcessInfoFromNvidiaSMI(ctx)
	if err != nil {
		return nil, fmt.Errorf("从 nvidia-smi 获取进程信息失败: %w", err)
	}

	if len(processList) == 0 {
		return []ProcessInfo{}, nil
	}

	var processInfos []ProcessInfo
	for _, processInfo := range processList {
		info, err := r.getProcessDetails(processInfo.PID, processInfo.GPUName, processInfo.ProcessName)
		if err != nil {
			r.processThrottle.Log("获取进程 id 为%v 详细信息失败:%v", processInfo.PID, err)
			continue
		}
		processInfos = append(processInfos, info)
	}
	return processInfos, nil
}

// getProcessInfoFromNvidiaSMI 直接从 nvidia-smi 获取进程部分信息
func (r *systemInfoRepository) getProcessInfoFromNvidiaSMI(ctx context.Context) ([]nvidiaSMIProcessInfo, error) {
	var processList []nvidiaSMIProcessInfo

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid,gpu_bus_id,process_name", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return processList, fmt.Errorf("执行 nvidia-smi 命令失败: %w", err)
	}

	gpuIndexMap := r.getGPUIndexMap(ctx)

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(strings.TrimSpace(line), ", ")
		if len(parts) >= 3 {
			pidStr := parts[0]
			busId := parts[1]
			processName := parts[2]

			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				r.gpuThrottle.Log("解析 PID 失败: %v", err)
				continue
			}

			displayName := gpuIndexMap[busId]
			if displayName == "" {
				displayName = busId
			}

			processList = append(processList, nvidiaSMIProcessInfo{
				PID:         pid,
				ProcessName: processName,
				GPUName:     displayName,
			})
		}
	}

	return processList, nil
}

// getGPUIndexMap 获取 GPU bus_id 到 index-name 的映射
func (r *systemInfoRepository) getGPUIndexMap(ctx context.Context) map[string]string {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=index,gpu_bus_id,name", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return make(map[string]string)
	}

	gpuMap := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) >= 3 {
			index := strings.TrimSpace(fields[0])
			busId := strings.TrimSpace(fields[1])
			name := strings.TrimSpace(fields[2])
			gpuMap[busId] = fmt.Sprintf("%s-%s", index, name)
		}
	}
	return gpuMap
}

// getProcessDetails 获取进程的详细信息
func (r *systemInfoRepository) getProcessDetails(pid int, gpuName string, processName string) (ProcessInfo, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return ProcessInfo{}, err
	}

	username, err := p.Username()
	if err != nil {
		r.processThrottle.Log("无法获取进程用户名: %v", err)
	}
	createTime, err := p.CreateTime()
	if err != nil {
		r.processThrottle.Log("无法获取进程创建时间: %v", err)
	}
	cmdline, err := p.Cmdline()
	if err != nil {
		r.processThrottle.Log("无法获取进程命令行: %v", err)
	}

	startTime := time.Unix(createTime/1000, 0).Format("2006-01-02 15:04:05")
	duration := time.Since(time.Unix(createTime/1000, 0))
	runtime := formatDuration(duration)

	isNormal := 1
	if running, err := p.IsRunning(); err != nil || !running {
		isNormal = 0
	}

	return ProcessInfo{
		Username:            username,
		PID:                 strconv.Itoa(pid),
		JobName:             processName,
		GPUname:             gpuName,
		StartTime:           startTime,
		IsNormal:            isNormal,
		Runtime:             runtime,
		RunningDurationSecs: int(duration.Seconds()),
		Command:             cmdline,
	}, nil
}

// bytesToGB 转换字节为 GB 字符串
func bytesToGB(bytes uint64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1fGB", gb)
}

// formatDuration 格式化时间间隔
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
