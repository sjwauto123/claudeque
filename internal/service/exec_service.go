package service

import (
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
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

	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("ListCondaEnvs: 获取 SSH 客户端失败: userID=%d, isRoot=%v, err=%v", userID, isRoot, err)
		return nil, err
	}

	// 尝试加载用户的环境变量配置，特别是 .bashrc 或 conda 的初始化脚本
	// 直接执行 conda env list，如果环境变量未配置好则尝试加载 .bashrc，最后兜底常见路径
	cmd := `conda env list 2>/dev/null || bash -lc 'conda env list' 2>/dev/null || bash -c 'source ~/.bashrc 2>/dev/null; conda env list' 2>/dev/null || /opt/conda/bin/conda env list 2>/dev/null || ~/miniconda3/bin/conda env list 2>/dev/null || ~/anaconda3/bin/conda env list 2>/dev/null || /usr/local/miniconda3/bin/conda env list 2>/dev/null || /usr/local/anaconda3/bin/conda env list 2>/dev/null || /root/miniconda3/bin/conda env list 2>/dev/null || /root/anaconda3/bin/conda env list 2>/dev/null`

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
		// conda env list 输出格式通常是: name   *   /path/to/env 或者 name /path/to/env
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

	// 构建环境变量前缀
	var envPrefix strings.Builder
	for k, v := range envVars {
		envPrefix.WriteString(fmt.Sprintf("export %s=%s && ", k, v))
	}

	jobID := envVars["JOB_ID"]
	logDir := "$HOME/cloudque_logs"
	logFile := fmt.Sprintf("%s/job_%s.log", logDir, jobID)

	// 构建执行命令
	var runCmd string
	if condaEnv != "" {
		// 添加常见的 conda 路径到 PATH 中，确保能找到 conda 命令
		condaPath := `export PATH=$PATH:/opt/conda/bin:$HOME/miniconda3/bin:$HOME/anaconda3/bin:/usr/local/miniconda3/bin:/usr/local/anaconda3/bin:/root/miniconda3/bin:/root/anaconda3/bin; `
		runCmd = fmt.Sprintf("%sconda run --no-capture-output -n %s %s", condaPath, condaEnv, cmdStr)
	} else {
		runCmd = cmdStr
	}

	// 模拟 cmd.Process.Pid 的核心逻辑：
	// 1. 使用 bash -c 启动，先打印当前 Shell 的 PID ($$)
	// 2. 然后使用 exec 执行 nohup 命令，让任务进程替换掉当前的 Shell 进程
	// 3. 这样任务进程就会继承刚才打印出来的那个 PID
	fullCmd := fmt.Sprintf("mkdir -p %s && bash -c 'echo $$ ; %s exec nohup %s > %s 2>&1'",
		logDir, envPrefix.String(), runCmd, logFile)

	logger.Info("模拟 cmd.Process.Pid 逻辑启动任务", zap.String("job_id", jobID))

	// 1. 获取底层的 ssh.Client 并开启 Session
	sshClient := client.GetSSHClient()
	session, err := sshClient.NewSession()
	if err != nil {
		return 0, fmt.Errorf("创建 SSH 会话失败: %w", err)
	}

	// 2. 获取输出管道以读取 PID
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return 0, fmt.Errorf("获取 StdoutPipe 失败: %w", err)
	}

	// 3. 异步启动命令（对应本地的 cmd.Start()）
	if err := session.Start(fullCmd); err != nil {
		session.Close()
		return 0, fmt.Errorf("启动远程命令失败: %w", err)
	}

	// 4. 立即从输出流读取 PID（对应本地的 cmd.Process.Pid）
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

	// 5. 等待结果，确保拿到 PID
	select {
	case res := <-resChan:
		if res.err != nil {
			session.Close()
			return 0, fmt.Errorf("读取 PID 失败: %w", res.err)
		}
		// 拿到 PID 后，我们不需要关闭 session，
		// 而是让它在后台运行。为了防止 session 结束导致进程收到 SIGHUP，
		// 我们之前已经在命令里加了 nohup。
		// 这里我们启动一个协程去等待 session 结束，释放资源
		go func() {
			_ = session.Wait()
			session.Close()
		}()
		return res.pid, nil
	case <-time.After(15 * time.Second):
		session.Close()
		return 0, fmt.Errorf("获取远程 PID 超时")
	}
}

// IsProcessRunningRemote 检查远程进程是否在运行
func (s *execService) IsProcessRunningRemote(ctx context.Context, userID int, pid int, isRoot bool) (bool, error) {
	client, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("IsProcessRunningRemote: 获取 SSH 客户端失败: userID=%d, pid=%d, err=%v", userID, pid, err)
		return false, err
	}

	// 使用 kill -0 PID 检查进程是否存在
	cmd := fmt.Sprintf("kill -0 %d", pid)
	_, exitCode, err := client.ExecuteCommandWithStatus(cmd)
	if err != nil {
		if exitCode == 1 {
			// 进程不存在，这是正常情况
			logger.Debugf("IsProcessRunningRemote: 进程不存在: userID=%d, pid=%d", userID, pid)
			return false, nil
		}
		// 其他错误（如连接断开）
		logger.Errorf("IsProcessRunningRemote: 执行检查命令失败: userID=%d, pid=%d, err=%v", userID, pid, err)
		return false, err
	}

	return true, nil
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
