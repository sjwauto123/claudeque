package service

import (
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type execService struct {
	sessionManager *ssh.SessionManager
	authService    AuthService
}

// NewExecService 创建执行服务
func NewExecService(sessionManager *ssh.SessionManager, authService AuthService) ExecService {
	return &execService{
		sessionManager: sessionManager,
		authService:    authService,
	}
}

// ListCondaEnvs 获取远程服务器上的 Conda 环境列表
func (s *execService) ListCondaEnvs(ctx context.Context, userID int) ([]string, error) {
	// 检查用户是否拥有 Root 权限
	isRoot, err := s.authService.HasSystemAccess(userID, AccessTypeFile)
	if err != nil {
		logger.Errorf("ListCondaEnvs: 检查用户权限失败: userID=%d, err=%v", userID, err)
		isRoot = false // 回退到普通用户权限
	}
	// 获取SSH连接
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("ListCondaEnvs: 获取 SSH 客户端失败: userID=%d, isRoot=%v, err=%v", userID, isRoot, err)
		return nil, err
	}
	// 尝试加载Bash配置文件，失败静默处理，然后级联路径查找Conda环境
	cmd := "source ~/.bashrc 2>/dev/null; conda env list || /opt/conda/bin/conda env list || ~/miniconda3/bin/conda env list || ~/anaconda3/bin/conda env list"
	// 执行命令
	output, err := client.ExecuteCommand(cmd)
	if err != nil {
		logger.Errorf("ListCondaEnvs: 执行 conda env list 失败: userID=%d, err=%v, output=%s", userID, err, output)
		return nil, fmt.Errorf("conda命令不存在")
	}
	// 解析输出
	lines := strings.Split(output, "\n")
	var envs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) > 0 {
			envs = append(envs, parts[0])
		}
	}
	if len(envs) == 0 {
		return nil, fmt.Errorf("conda命令不存在")
	}
	return envs, nil
}

// ExecuteCommandRemote 在指定环境下远程执行命令，返回 PID
func (s *execService) ExecuteCommandRemote(ctx context.Context, userID int, condaEnv string, cmdStr string, envVars map[string]string, isRoot bool) (int, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("ExecuteCommandRemote: 获取 SSH 客户端失败: userID=%d, err=%v", userID, err)
		return 0, err
	}
	// 将环境变量转换成标准的 Shell 环境变量导出语句
	var envPrefix strings.Builder
	for k, v := range envVars {
		envPrefix.WriteString(fmt.Sprintf("export %s=%s && ", k, v))
	}
	// 构建文件路径
	jobID, logDir := envVars["JOB_ID"], "$HOME/cloudque_logs"
	logFile := fmt.Sprintf("%s/job_%s.log", logDir, jobID)
	exitFile := fmt.Sprintf("%s/job_%s.exitcode", logDir, jobID)
	// 构建执行命令
	var runCmd string
	runCmd = fmt.Sprintf("conda run --no-capture-output -n %s %s", condaEnv, cmdStr)
	if condaEnv != "" {
		runCmd = fmt.Sprintf("conda run --no-capture-output -n %s %s", condaEnv, cmdStr)
	} else {
		runCmd = cmdStr
	}
	fullCmd := fmt.Sprintf("mkdir -p %s && bash -l -c 'echo $$ ; %s exec nohup bash -c \"%s; echo \\$? > %s\" > %s 2>&1'",
		logDir, envPrefix.String(), runCmd, exitFile, logFile)
	// 开启 Session
	session, err := client.GetSSHClient().NewSession()
	if err != nil {
		return 0, fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	// 获取输出管道以读取 PID
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return 0, fmt.Errorf("获取 StdoutPipe 失败: %w", err)
	}
	// 异步启动命令
	if err := session.Start(fullCmd); err != nil {
		session.Close()
		return 0, fmt.Errorf("启动远程命令失败: %w", err)
	}
	// 从输出流读取 PID
	type result struct {
		pid int
		err error
	}
	resChan := make(chan result, 1)
	go func() {
		var pid int
		if _, err := fmt.Fscanf(stdout, "%d", &pid); err != nil {
			resChan <- result{0, err}
			return
		}
		resChan <- result{pid, nil}
	}()
	// 拿到 PID
	select {
	case res := <-resChan:
		if res.err != nil {
			session.Close()
			return 0, fmt.Errorf("读取 PID 失败: %w", res.err)
		}

		// 异步等待 session 结束
		go func() {
			_ = session.Wait()
			session.Close()
		}()

		// 尝试查找真正的业务子进程 PID (例如 python)
		// 给进程一点启动时间
		time.Sleep(500 * time.Millisecond)
		workerPID, err := s.findWorkerPID(userID, res.pid, isRoot)
		if err != nil {
			logger.Warnf("查找业务子进程失败, 使用原始PID: %v", err)
			return res.pid, nil
		}

		return workerPID, nil
	case <-time.After(15 * time.Second):
		session.Close()
		return 0, fmt.Errorf("获取远程 PID 超时")
	}
}

// FileExistsRemote 检查远程文件是否存在
func (s *execService) FileExistsRemote(ctx context.Context, userID int, filePath string, isRoot bool) (bool, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("FileExistsRemote: 获取 SSH 客户端失败: userID=%d, path=%s, err=%v", userID, filePath, err)
		return false, err
	}

	// 使用 test -f 检查文件是否存在
	cmd := fmt.Sprintf("test -f %s", filePath)
	_, exitCode, err := client.ExecuteCommandWithStatus(cmd)
	if err != nil {
		if exitCode == 1 {
			// 文件不存在
			return false, nil
		}
		// 其他错误
		logger.Errorf("FileExistsRemote: 执行检查命令失败: userID=%d, path=%s, err=%v", userID, filePath, err)
		return false, err
	}

	return true, nil
}

// GetJobExitCode 获取任务的退出码
func (s *execService) GetJobExitCode(ctx context.Context, userID int, jobID int, isRoot bool) (int, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("GetJobExitCode: 获取 SSH 客户端失败: userID=%d, jobID=%d, err=%v", userID, jobID, err)
		return -1, err
	}

	exitFile := fmt.Sprintf("$HOME/cloudque_logs/job_%d.exitcode", jobID)
	cmd := fmt.Sprintf("cat %s", exitFile)

	// 重试机制：由于进程退出（PID消失）和包装器 Bash 将退出码写入文件之间存在极小的时间差，
	// 所以需要重试几次以确保能读到文件
	var output string
	var execErr error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		output, execErr = client.ExecuteCommand(cmd)
		if execErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if execErr != nil {
		return -1, fmt.Errorf("读取退出码文件失败 (已重试 %d 次): %w", maxRetries, execErr)
	}

	output = strings.TrimSpace(output)
	exitCode, err := strconv.Atoi(output)
	if err != nil {
		return -1, fmt.Errorf("解析退出码失败: %w, 内容: %s", err, output)
	}

	return exitCode, nil
}

// getSSHClient 获取用户的 SSH 客户端
func (s *execService) getSSHClient(userID int, isRoot bool) (*server.Client, error) {
	if s.sessionManager == nil {
		return nil, fmt.Errorf("SSH 会话管理器未初始化")
	}
	session, err := s.sessionManager.GetSession(userID, isRoot)
	if err != nil || session == nil || session.Client == nil {
		// 尝试恢复会话
		if err := s.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
			return nil, fmt.Errorf("恢复 SSH 会话失败: %w", err)
		}
		session, err = s.sessionManager.GetSession(userID, isRoot)
		if err != nil || session == nil || session.Client == nil {
			return nil, fmt.Errorf("获取 SSH 客户端失败")
		}
	}
	return session.Client, nil
}

// findWorkerPID 递归查找叶子节点进程（例如真实的 python 训练进程）
func (s *execService) findWorkerPID(userID int, parentPID int, isRoot bool) (int, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return parentPID, err
	}

	// 这是一个递归查找叶子节点的脚本
	// 我们查找运行时间最长（通常是 python）或者最深层的子进程
	// pgrep -P 会列出子进程 PID
	script := fmt.Sprintf(`
		find_worker() {
			local pid=$1
			# 查找所有子进程
			local children=$(pgrep -P $pid 2>/dev/null)
			if [ -z "$children" ]; then
				echo $pid
				return
			fi
			# 如果有子进程，递归查找。如果有多个，优先找包含 python 的进程名
			for child in $children; do
				if ps -p $child -o comm= 2>/dev/null | grep -qi python; then
					find_worker $child
					return
				fi
			done
			# 如果没找到 python，就找最后一个子进程（通常是最后启动的）
			last_child=$(echo $children | awk '{print $NF}')
			find_worker $last_child
		}
		find_worker %d
	`, parentPID)

	output, exitCode, err := client.ExecuteCommandWithStatus(script)
	if err != nil {
		if exitCode != 0 {
			// 如果脚本执行出错（比如进程已消失），返回原 PID
			return parentPID, nil
		}
		return parentPID, err
	}

	childPIDStr := strings.TrimSpace(output)
	if childPIDStr == "" {
		return parentPID, nil
	}

	childPID, err := strconv.Atoi(childPIDStr)
	if err != nil {
		return parentPID, nil // 转换失败返回原 PID
	}

	return childPID, nil
}
