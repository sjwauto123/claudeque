package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"cloudque/pkg/logger"
	"cloudque/pkg/ssh"
)

type execService struct {
	sessionManager *ssh.SessionManager
	authService    AuthService
}

func NewExecService(sessionManager *ssh.SessionManager, authService AuthService) ExecService {
	return &execService{sessionManager: sessionManager, authService: authService}
}

func (s *execService) ensureSession(userID int) (*ssh.UserSession, error) {
	// 优先获取现有会话，不存在则通过AuthService恢复
	session, err := s.sessionManager.GetSession(userID, false)
	if err != nil || session == nil || session.Client == nil {
		if err := s.authService.EnsureSSHSession(userID); err != nil {
			return nil, fmt.Errorf("恢复SSH会话失败: %w", err)
		}
		session, err = s.sessionManager.GetSession(userID, false)
		if err != nil || session == nil || session.Client == nil {
			return nil, fmt.Errorf("获取SSH会话失败")
		}
	}
	return session, nil
}

func (s *execService) RunBackgroundForUser(ctx context.Context, userID int, cmd string, workdir string, env map[string]string) (int, error) {
	session, err := s.ensureSession(userID)
	if err != nil {
		return 0, err
	}
	jobID := env["JOB_ID"]
	var exitFile string
	var logFile string
	var pidFile string
	if jobID != "" {
		username := session.GetUsername()
		home := "/home/" + username
		exitFile = home + "/cloudque_exit/job_" + jobID + ".code"
		logFile = home + "/cloudque_logs/job_" + jobID + ".log"
		pidFile = home + "/cloudque_exit/job_" + jobID + ".pid"
	}
	var exports []string
	for k, v := range env {
		escaped := strings.ReplaceAll(v, "'", "'\\''")
		exports = append(exports, fmt.Sprintf("export %s='%s'", k, escaped))
	}
	sort.Strings(exports)
	var parts []string
	if workdir != "" {
		parts = append(parts, "cd "+escapeBashArg(workdir))
	}
	if len(exports) > 0 {
		parts = append(parts, strings.Join(exports, "; "))
	}
	if exitFile != "" {
		parts = append(parts, "mkdir -p $(dirname "+escapeBashArg(exitFile)+") || true")
	}
	if pidFile != "" {
		parts = append(parts, "mkdir -p $(dirname "+escapeBashArg(pidFile)+") || true")
	}
	if logFile != "" {
		parts = append(parts, "mkdir -p $(dirname "+escapeBashArg(logFile)+") || true")
	}
	parts = append(parts, cmd)
	full := strings.Join(parts, " && ")
	if exitFile != "" {
		full = full + " ; code=$?; echo $code > " + escapeBashArg(exitFile)
		if logFile != "" {
			full = full + " ; echo EXIT_CODE: $code >> " + escapeBashArg(logFile)
		}
	}
	if logFile != "" {
		prefix := "mkdir -p $(dirname " + escapeBashArg(logFile) + ") || true; mkdir -p $(dirname " + escapeBashArg(exitFile) + ") || true; mkdir -p $(dirname " + escapeBashArg(pidFile) + ") || true; "
		inner := "(" + full + ") >> " + escapeBashArg(logFile) + " 2>&1"
		wrapper := "bash -lc \"" + prefix + "nohup sh -c " + escapeBashArg(inner) + " </dev/null & pid=\\$!; echo \\$pid; echo \\$pid > " + escapeBashArg(pidFile) + "\""
		out, err := session.Client.ExecuteCommand(wrapper)
		if err != nil {
			appendCmd := "bash -lc \"mkdir -p $(dirname " + escapeBashArg(logFile) + "); echo START_FAILED: $(date) >> " + escapeBashArg(logFile) + "\""
			_, _ = session.Client.ExecuteCommand(appendCmd)
			return 0, fmt.Errorf("后台启动失败: %w", err)
		}
		pid := parsePID(out)
		if pid <= 0 {
			if pidFile != "" {
				readPidCmd := "bash -lc \"cat " + escapeBashArg(pidFile) + "\""
				pout, perr := session.Client.ExecuteCommand(readPidCmd)
				if perr == nil {
					pid = parsePID(pout)
				}
			}
			if pid <= 0 {
				logger.Warnf("未解析到PID，输出: %s", out)
				return 0, fmt.Errorf("未获取到后台进程PID")
			}
		}
		return pid, nil
	}
	inner := "(" + full + ") >/dev/null 2>&1"
	wrapper := "bash -lc \"mkdir -p $(dirname " + escapeBashArg(exitFile) + ") || true; mkdir -p $(dirname " + escapeBashArg(pidFile) + ") || true; nohup sh -c " + escapeBashArg(inner) + " </dev/null & pid=\\$!; echo \\$pid; echo \\$pid > " + escapeBashArg(pidFile) + "\""
	out, err := session.Client.ExecuteCommand(wrapper)
	if err != nil {
		return 0, fmt.Errorf("后台启动失败: %w", err)
	}
	pid := parsePID(out)
	if pid <= 0 {
		if pidFile != "" {
			readPidCmd := "bash -lc \"cat " + escapeBashArg(pidFile) + "\""
			pout, perr := session.Client.ExecuteCommand(readPidCmd)
			if perr == nil {
				pid = parsePID(pout)
			}
		}
		if pid <= 0 {
			logger.Warnf("未解析到PID，输出: %s", out)
			return 0, fmt.Errorf("未获取到后台进程PID")
		}
	}
	return pid, nil
}

func (s *execService) IsProcessRunning(ctx context.Context, userID int, pid int) (bool, error) {
	session, err := s.ensureSession(userID)
	if err != nil {
		return false, err
	}
	// 优先使用 /proc/<pid> 判断进程是否存在，避免 EPERM 误判
	check := fmt.Sprintf("bash -lc \"[ -d /proc/%d ] && echo alive || echo dead\"", pid)
	out, err := session.Client.ExecuteCommand(check)
	if err != nil {
		return false, err
	}
	out = strings.TrimSpace(out)
	return strings.Contains(out, "alive"), nil
}

func (s *execService) GetExitCode(ctx context.Context, userID int, jobID int) (int, error) {
	session, err := s.ensureSession(userID)
	if err != nil {
		return 0, err
	}
	username := session.GetUsername()
	path := fmt.Sprintf("/home/%s/cloudque_exit/job_%d.code", username, jobID)
	cmd := "bash -lc \"cat " + escapeBashArg(path) + "\""
	out, err := session.Client.ExecuteCommand(cmd)
	if err != nil {
		return 0, err
	}
	codeStr := strings.TrimSpace(out)
	var code int
	if _, scanErr := fmt.Sscanf(codeStr, "%d", &code); scanErr != nil {
		return 0, scanErr
	}
	return code, nil
}

func escapeBashArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func parsePID(out string) int {
	re := regexp.MustCompile(`\b(\d{2,})\b`)
	m := re.FindStringSubmatch(strings.TrimSpace(out))
	if len(m) >= 2 {
		var pid int
		fmt.Sscanf(m[1], "%d", &pid)
		return pid
	}
	return 0
}
