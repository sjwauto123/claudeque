package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GpuInfo GPU信息
type GpuInfo struct {
	Index  int
	UUID   string
	Name   string
	Type   string
	Memory int // 单位: MB
}

// GetGpuInfo 获取服务器上的GPU信息
func GetGpuInfo() ([]GpuInfo, error) {
	if runtime.GOOS == "windows" {
		// Windows 环境下返回模拟数据用于测试
		return []GpuInfo{
			{Index: 0, UUID: "GPU-mock-uuid-001", Name: "gpu-0", Type: "NVIDIA GeForce RTX 3080 (Mock)", Memory: 10240},
		}, nil
	}

	// Linux 环境下执行 nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,uuid,gpu_name,memory.total", "--format=csv,noheader,nounits")
	// 返回示例：0, GPU-3c1b8f2d-6a44-9e2f-9b7e-1f0d0a123456, NVIDIA GeForce RTX 3090, 24576
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// 如果没有 nvidia-smi，可能是没有驱动或者不是 NVIDIA 环境，返回空或错误
		return nil, fmt.Errorf("执行 nvidia-smi 失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var gpus []GpuInfo
	for lineNum, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			return nil, fmt.Errorf("解析 nvidia-smi 输出失败，第 %d 行字段不足: %q", lineNum+1, line)
		}

		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("解析 GPU 显存失败，第 %d 行: %q: %w", lineNum+1, index, err)
		}
		uuid := strings.TrimSpace(parts[1])
		name := fmt.Sprintf("gpu-%d", index)
		gpuType := strings.TrimSpace(parts[2])
		memory, err := strconv.Atoi(strings.TrimSpace(parts[3]))
		if err != nil {
			return nil, fmt.Errorf("解析 GPU 显存失败，第 %d 行: %q: %w", lineNum+1, index, err)
		}

		gpus = append(gpus, GpuInfo{
			Index:  index,
			UUID:   uuid,
			Name:   name,
			Type:   gpuType,
			Memory: memory,
		})
	}

	return gpus, nil
}
