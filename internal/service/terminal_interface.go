package service

import (
	"cloudque/pkg/server"
	"io"
)

// TerminalService 终端服务接口
type TerminalService interface {
	// NewTerminalSession 创建一个新的终端会话，返回会话对象
	NewTerminalSession(userID int, stdin io.Reader, stdout, stderr io.Writer, isRoot bool, cols, rows int) (*server.TerminalSession, error)
	// GetOrCreateTerminal 获取或创建持久化终端
	GetOrCreateTerminal(userID int, isRoot bool, cols, rows int) (*PersistentTerminal, error)
	// ResizeTerminal 调整终端大小
	ResizeTerminal(userID int, isRoot bool, cols, rows int) error
	// CloseTerminal 关闭终端
	CloseTerminal(userID int, isRoot bool) error
}

// PersistentTerminal 持久化终端结构
type PersistentTerminal struct {
	ID         string
	UserID     int
	IsRoot     bool
	Session    *server.TerminalSession
	Stdin      io.WriteCloser // 用于写入输入
	OutputChan chan []byte    // 输出通道
	History    []byte         // 历史记录
}
