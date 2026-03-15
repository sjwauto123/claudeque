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
	parts = append(parts, cmd)
	full := strings.Join(parts, " && ")
	wrapper := "bash -lc '(" + full + ") >/dev/null 2>&1 & echo $!'"
	out, err := session.Client.ExecuteCommand(wrapper)
	if err != nil {
		return 0, fmt.Errorf("后台启动失败: %w", err)
	}
	pid := parsePID(out)
	if pid <= 0 {
		logger.Warnf("未解析到PID，输出: %s", out)
		return 0, fmt.Errorf("未获取到后台进程PID")
	}
	return pid, nil
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
