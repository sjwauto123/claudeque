package repository

import (
	"cloudque/internal/model/dto/response"
	"context"
)

// ProcessInfo 进程信息（仓储层内部使用）
type ProcessInfo struct {
	Username  string
	PID       string
	JobName   string
	GPUname   string
	StartTime string
	IsNormal  int
	Runtime   string
	Command   string
}

// SystemInfoRepository 系统信息仓储接口
type SystemInfoRepository interface {
	// GetDiskInfo 获取磁盘信息
	GetDiskInfo(ctx context.Context, mountPoint string) (*response.CpuInfoResponse, error)
	// GetMemoryInfo 获取内存信息
	GetMemoryInfo(ctx context.Context) (*response.CpuInfoResponse, error)
	// GetGPUInfo 获取 GPU 信息
	GetGPUInfo(ctx context.Context) ([]response.GPUInfoResponse, map[string]string, error)
	// GetProcessInfo 获取进程信息
	GetProcessInfo(ctx context.Context, gpuMap map[string]string) ([]ProcessInfo, error)
}
