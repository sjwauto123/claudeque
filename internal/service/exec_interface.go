package service

import (
	"context"
)

// ExecService 执行服务接口，处理 Conda 环境和远程 SSH 执行
type ExecService interface {
	// ListCondaEnvs 获取远程服务器上的 Conda 环境列表
	ListCondaEnvs(ctx context.Context, userID int) ([]string, error)
	// ExecuteCommandRemote 在指定环境下远程执行命令，返回 PID
	ExecuteCommandRemote(ctx context.Context, userID int, condaEnv string, cmd string, envVars map[string]string) (int, error)
	// IsProcessRunningRemote 检查远程进程是否在运行
	IsProcessRunningRemote(ctx context.Context, userID int, pid int) (bool, error)
	// FileExistsRemote 检查远程文件是否存在
	FileExistsRemote(ctx context.Context, userID int, filePath string) (bool, error)
}
