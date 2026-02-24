package service

import (
	pkgerrors "cloudque/pkg/errors"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"io"
)

// terminalService 终端服务实现
type terminalService struct {
	sessionManager *ssh.SessionManager
}

// NewTerminalService 创建终端服务
func NewTerminalService(sessionManager *ssh.SessionManager) TerminalService {
	return &terminalService{
		sessionManager: sessionManager,
	}
}

// getSSHClient 获取用户的SSH客户端
func (s *terminalService) getSSHClient(userID int, isRoot bool) (*server.Client, error) {
	if s.sessionManager == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternalError, "SSH会话管理器未初始化")
	}

	session, err := s.sessionManager.GetSession(userID, isRoot)
	if err != nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternalError, "获取SSH会话失败，请重新登录")
	}

	return session.Client, nil
}

// RunInteractiveSession 运行交互式会话（PTY 透传）
func (s *terminalService) RunInteractiveSession(userID int, stdin io.Reader, stdout, stderr io.Writer, isRoot bool) error {
	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return err
	}
	return sshClient.RunInteractiveSession(stdin, stdout, stderr)
}

// ResizePTY 调整窗口大小
func (s *terminalService) ResizePTY(userID int, cols, rows int, isRoot bool) error {
	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return err
	}
	return sshClient.ResizePTY(cols, rows)
}
