package service

import (
	"bytes"
	pkgerrors "cloudque/pkg/errors"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"fmt"
	"io"
	"sync"
	"time"

	"go.uber.org/zap"
)

// persistentTerminalImpl 持久化终端内部实现
type persistentTerminalImpl struct {
	ID         string
	UserID     int
	IsRoot     bool
	Session    *server.TerminalSession
	OutputChan chan []byte // 广播通道
	History    *bytes.Buffer
	LastActive time.Time
	mu         sync.Mutex
	closed     bool

	// 用于向 Session 写入数据
	stdinWriter io.WriteCloser
}

// terminalService 终端服务实现
type terminalService struct {
	sessionManager *ssh.SessionManager
	authService    AuthService

	terminals map[string]*persistentTerminalImpl
	mu        sync.RWMutex
}

// NewTerminalService 创建终端服务
func NewTerminalService(sessionManager *ssh.SessionManager, authService AuthService) TerminalService {
	ts := &terminalService{
		sessionManager: sessionManager,
		authService:    authService,
		terminals:      make(map[string]*persistentTerminalImpl),
	}

	// 启动清理任务
	go ts.cleanupLoop()

	return ts
}

// getSSHClient 获取用户的SSH客户端
func (s *terminalService) getSSHClient(userID int, isRoot bool) (*server.Client, error) {
	if s.sessionManager == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternalError, "SSH会话管理器未初始化")
	}

	session, err := s.sessionManager.GetSession(userID, isRoot)

	// 检查连接是否存活
	if err == nil && session != nil && session.Client != nil {
		if _, err := session.Client.ExecuteCommand("echo 1"); err == nil {
			return session.Client, nil
		}
	}

	// 尝试恢复
	if err := s.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "恢复SSH会话失败", err)
	}

	// 重新获取
	session, err = s.sessionManager.GetSession(userID, isRoot)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "获取SSH会话失败", err)
	}

	return session.Client, nil
}

// NewTerminalSession 创建新的交互式会话 (旧接口，保留兼容性但不再推荐使用)
func (s *terminalService) NewTerminalSession(userID int, stdin io.Reader, stdout, stderr io.Writer, isRoot bool, cols, rows int) (*server.TerminalSession, error) {
	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return nil, err
	}
	return sshClient.NewTerminalSession(stdin, stdout, stderr, cols, rows)
}

// TerminalWriter 辅助写入器，用于捕获输出并写入历史和广播
type TerminalWriter struct {
	pt *persistentTerminalImpl
}

func (w *TerminalWriter) Write(p []byte) (int, error) {
	w.pt.mu.Lock()
	defer w.pt.mu.Unlock()

	if w.pt.closed {
		return 0, io.ErrClosedPipe
	}

	w.pt.LastActive = time.Now()

	// 写入历史 (限制大小 50KB)
	const MaxHistorySize = 50 * 1024
	if w.pt.History.Len()+len(p) > MaxHistorySize {
		// 如果超出限制，保留最近的 25KB (避免频繁分配)
		current := w.pt.History.Bytes()
		// 计算需要保留的起始位置
		keepStart := len(current) - (MaxHistorySize / 2)
		if keepStart < 0 {
			keepStart = 0
		}
		// 重建 Buffer
		w.pt.History = bytes.NewBuffer(current[keepStart:])
	}
	w.pt.History.Write(p)

	// 广播
	// 必须复制数据，因为 p 的底层数组可能会被重用
	data := make([]byte, len(p))
	copy(data, p)

	select {
	case w.pt.OutputChan <- data:
	default:
		// 如果通道满，丢弃数据以防止阻塞 SSH 会话
	}

	return len(p), nil
}

// GetOrCreateTerminal 获取或创建持久化终端
func (s *terminalService) GetOrCreateTerminal(userID int, isRoot bool, cols, rows int) (*PersistentTerminal, error) {
	key := fmt.Sprintf("%d:%v", userID, isRoot)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 尝试获取现有终端
	if pt, exists := s.terminals[key]; exists {
		pt.mu.Lock()
		defer pt.mu.Unlock()

		pt.LastActive = time.Now()

		// 调整大小
		if pt.Session != nil {
			_ = pt.Session.Resize(cols, rows)
		}

		// 返回副本以避免并发读写问题
		historyCopy := make([]byte, pt.History.Len())
		copy(historyCopy, pt.History.Bytes())

		// 返回公共结构
		return &PersistentTerminal{
			ID:         pt.ID,
			UserID:     pt.UserID,
			IsRoot:     pt.IsRoot,
			Session:    pt.Session,
			Stdin:      pt.stdinWriter,
			OutputChan: pt.OutputChan,
			History:    historyCopy,
		}, nil
	}

	// 2. 创建新终端
	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		return nil, err
	}

	pt := &persistentTerminalImpl{
		ID:         key,
		UserID:     userID,
		IsRoot:     isRoot,
		OutputChan: make(chan []byte, 1024), // 增大缓冲区
		History:    bytes.NewBuffer(make([]byte, 0, 1024)),
		LastActive: time.Now(),
	}

	writer := &TerminalWriter{pt: pt}

	// 创建管道用于 stdin
	stdinReader, stdinWriter := io.Pipe()
	pt.stdinWriter = stdinWriter

	// 创建会话，将 stdout/stderr 指向自定义 Writer
	session, err := sshClient.NewTerminalSession(stdinReader, writer, writer, cols, rows)
	if err != nil {
		_ = stdinWriter.Close()
		return nil, err
	}

	pt.Session = session
	s.terminals[key] = pt

	// 监控会话退出
	go func() {
		_ = session.Session.Wait()
		err := s.CloseTerminal(userID, isRoot)
		if err != nil {
			return
		}
	}()

	// 返回副本
	historyCopy := make([]byte, pt.History.Len())
	copy(historyCopy, pt.History.Bytes())

	return &PersistentTerminal{
		ID:         pt.ID,
		UserID:     pt.UserID,
		IsRoot:     pt.IsRoot,
		Session:    pt.Session,
		Stdin:      pt.stdinWriter,
		OutputChan: pt.OutputChan,
		History:    historyCopy,
	}, nil
}

// ResizeTerminal 调整终端大小
func (s *terminalService) ResizeTerminal(userID int, isRoot bool, cols, rows int) error {
	key := fmt.Sprintf("%d:%v", userID, isRoot)
	s.mu.RLock()
	pt, exists := s.terminals[key]
	s.mu.RUnlock()

	if !exists {
		return pkgerrors.New(pkgerrors.CodeNotFound, "终端会话不存在")
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.LastActive = time.Now()

	if pt.Session != nil {
		return pt.Session.Resize(cols, rows)
	}
	return nil
}

// CloseTerminal 关闭终端
func (s *terminalService) CloseTerminal(userID int, isRoot bool) error {
	key := fmt.Sprintf("%d:%v", userID, isRoot)
	s.mu.Lock()
	pt, exists := s.terminals[key]
	if exists {
		delete(s.terminals, key)
	}
	s.mu.Unlock()

	if !exists {
		return nil
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	if !pt.closed {
		pt.closed = true
		if pt.Session != nil {
			_ = pt.Session.Close()
		}
		if pt.stdinWriter != nil {
			_ = pt.stdinWriter.Close()
		}
		close(pt.OutputChan)
	}

	return nil
}

// cleanupLoop 定期清理过期终端
func (s *terminalService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	timeout := 30 * time.Minute // 30分钟无操作自动断开

	for range ticker.C {
		s.mu.Lock()
		for key, pt := range s.terminals {
			pt.mu.Lock()
			if time.Since(pt.LastActive) > timeout {
				// 过期清理
				if !pt.closed {
					pt.closed = true
					if pt.Session != nil {
						_ = pt.Session.Close()
					}
					if pt.stdinWriter != nil {
						_ = pt.stdinWriter.Close()
					}
					close(pt.OutputChan)
				}
				delete(s.terminals, key)
				zap.L().Info("清理过期终端会话", zap.String("id", key))
			}
			pt.mu.Unlock()
		}
		s.mu.Unlock()
	}
}
