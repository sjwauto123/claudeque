package service

import (
	gwebsocket "github.com/gorilla/websocket"
)

// TerminalService 终端服务接口
type TerminalService interface {
	// HandleTerminalConnection 处理新的 websocket 连接，并将其绑定到对应的 SSH PTY 会话
	HandleTerminalConnection(userID int, isRoot bool, cols, rows int, conn *gwebsocket.Conn) error
	// ResizeTerminal 调整终端大小
	ResizeTerminal(userID int, isRoot bool, cols, rows int) error
}
