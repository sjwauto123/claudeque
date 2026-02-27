package service

import (
	"io"
)

// TerminalService 终端服务接口
type TerminalService interface {
	// RunInteractiveSession 运行交互式会话（用于 WebSocket 终端透传），阻塞直到会话结束
	RunInteractiveSession(userID int, stdin io.Reader, stdout, stderr io.Writer, isRoot bool) error
}
