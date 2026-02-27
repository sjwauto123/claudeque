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

// NewTerminalSession 创建新的交互式会话
func (s *terminalService) NewTerminalSession(userID int, stdin io.Reader, stdout, stderr io.Writer, isRoot bool, cols, rows int) (*server.TerminalSession, error) {
	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return nil, err
	}
	return sshClient.NewTerminalSession(stdin, stdout, stderr, cols, rows)
}
