package service

import (
	"cloudque/pkg/server"
	"io"
)

// TerminalService 终端服务接口
type TerminalService interface {
	// NewTerminalSession 创建一个新的终端会话，返回会话对象
	NewTerminalSession(userID int, stdin io.Reader, stdout, stderr io.Writer, isRoot bool, cols, rows int) (*server.TerminalSession, error)
}
