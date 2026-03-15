package service

import (
	"context"
)

type ExecService interface {
	// RunBackgroundForUser 通过SSH在指定用户上下文后台运行命令，返回PID
	// cmd: 需要在bash -lc中执行的命令片段（不含cd/env导出/后台符），workdir: 工作目录，env: 额外环境变量
	RunBackgroundForUser(ctx context.Context, userID int, cmd string, workdir string, env map[string]string) (int, error)
	// IsProcessRunning 远程检查指定PID是否仍在运行
	IsProcessRunning(ctx context.Context, userID int, pid int) (bool, error)
	// GetExitCode 从远程退出码文件读取退出码
	GetExitCode(ctx context.Context, userID int, jobID int) (int, error)
}
