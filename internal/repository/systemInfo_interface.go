package repository

import (
	"cloudque/internal/model/dto/response"
	"context"
)

// ProcessInfo 进程信息（仓储层内部使用）
type ProcessInfo struct {
	Username            string
	PID                 string
	JobName             string
	GPUname             string
	StartTime           string
	IsNormal            int
	Runtime             string
	RunningDurationSecs int
	Command             string
}

// nvidiaSMIProcessInfo 保存从 nvidia-smi 获取的进程信息
type nvidiaSMIProcessInfo struct {
	PID         int
	ProcessName string
	GPUName     string
}

// ProcessDuration 进程运行时长（用于自动中断检查）
type ProcessDuration struct {
	PID                 string
	RunningDurationSecs int
}

// SystemInfoRepository 系统信息仓储接口
type SystemInfoRepository interface {
	// GetDiskInfo 获取磁盘信息
	GetDiskInfo(mountPoint string) (*response.CpuInfoResponse, error)
	// GetMemoryInfo 获取内存信息
	GetMemoryInfo() (*response.CpuInfoResponse, error)
	// GetGPUInfo 获取 GPU 信息
	GetGPUInfo(ctx context.Context) ([]response.GPUInfoResponse, error)
	// GetProcessInfo 获取进程信息
	GetProcessInfo(ctx context.Context) ([]ProcessInfo, error)
	// GetGPUPIDs 获取 GPU 上所有进程的 PID 集合
	GetGPUPIDs(ctx context.Context) (map[string]bool, error)
	// GetProcessDurations 获取 GPU 上所有进程的 PID 和运行时长（用于自动中断检查）
	GetProcessDurations(ctx context.Context) ([]ProcessDuration, error)
}
