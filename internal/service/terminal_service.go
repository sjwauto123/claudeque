package service

import (
	pkgerrors "cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"cloudque/pkg/websocket"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	gwebsocket "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// sshSessionWrapper 保存活动的 SSH 会话及其标准输入管道。
type sshSessionWrapper struct {
	ID          string                  //会话ID
	Session     *server.TerminalSession //终端会话
	stdinWriter io.WriteCloser
	LastActive  time.Time
	mu          sync.Mutex
	closed      bool
}

// terminalService 终端服务实现
type terminalService struct {
	sessionManager *ssh.SessionManager
	authService    AuthService
	wsPool         *websocket.ConnectionPool     // 使用通用 websocket 连接池
	sessions       map[string]*sshSessionWrapper // 底层的 SSH PTY 会话管理
	mu             sync.RWMutex
}

// NewTerminalService 创建终端服务
func NewTerminalService(sessionManager *ssh.SessionManager, authService AuthService, wsPool *websocket.ConnectionPool) TerminalService {
	ts := &terminalService{
		sessionManager: sessionManager,
		authService:    authService,
		wsPool:         wsPool,
		sessions:       make(map[string]*sshSessionWrapper),
	}
	// 清理循环仍然需要关闭空闲的 *SSH 会话*，而不是 websocket 连接。
	go ts.cleanupIdleSessions()
	return ts
}

// PrivateTerminalWriter 用于将输出直接发送给特定的 websocket 客户端的私有终端写入器
type PrivateTerminalWriter struct {
	client *websocket.Client
	mu     sync.Mutex
	buffer []byte
}

// Write 将数据写入私有终端写入器
// 它会尝试将数据分割为有效的 UTF-8 序列，避免发送不完整的字符。
func (w *PrivateTerminalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. 将新数据追加到缓存
	data := append(w.buffer, p...)
	w.buffer = nil

	// 2. 寻找最后一次有效的 UTF-8 边界
	n := len(data)
	cut := n

	// UTF-8 最大长度为 4 字节
	// 从末尾倒序扫描，最多检查 3 个字节
	for i := 0; i < 3 && i < n; i++ {
		b := data[n-1-i]

		if b&0x80 == 0 {
			break
		}

		if b&0xC0 == 0xC0 {
			req := 0
			if b&0xE0 == 0xC0 {
				req = 2
			} else if b&0xF0 == 0xE0 {
				req = 3
			} else if b&0xF8 == 0xF0 {
				req = 4
			}

			if i+1 < req {
				cut = n - 1 - i
			}
			break
		}
	}

	// 3. 分割数据
	toSend := data[:cut]
	w.buffer = data[cut:]

	if len(toSend) == 0 {
		return len(p), nil
	}

	// 4. 构造 JSON 消息
	msg := struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}{
		Type: "output",
		Data: string(toSend),
	}

	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}

	// 直接发送给特定客户端
	select {
	case w.client.Send <- jsonBytes:
	default:
		logger.Errorf("PrivateTerminalWriter.Write: %s发送缓冲区满，无法发送数据", w.client)
	}

	return len(p), nil
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

// HandleTerminalConnection 处理新的 websocket 连接，并将其绑定到对应的 SSH PTY 会话。
func (s *terminalService) HandleTerminalConnection(userID int, isRoot bool, cols, rows int, conn *gwebsocket.Conn) error {
	// 首先，确保底层 SSH 会话可用。
	if err := s.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "无法建立SSH会话", err)
	}

	// 准备一个线程安全的容器来持有 SSH 会话。
	// 因为我们需要先注册 WebSocket 客户端，然后再创建 SSH 会话（以避免丢失初始输出），
	// 所以消息处理程序在开始时可能访问不到 sw。
	var sw *sshSessionWrapper
	var swMu sync.RWMutex
	var pendingInput []byte                     // 缓冲初始化期间的输入
	var pendingResize *struct{ cols, rows int } // 缓冲初始化期间的 resize 事件

	// 定义此 websocket 连接的消息处理程序。
	messageHandler := func(client *websocket.Client, messageType int, data []byte) error {
		// 尝试解析为结构化消息（用于调整大小事件）。
		type inboundMsg struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}

		var msg inboundMsg
		if err := json.Unmarshal(data, &msg); err == nil && msg.Type != "" {
			switch msg.Type {
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					swMu.Lock()
					if sw != nil && !sw.closed && sw.Session != nil {
						_ = sw.Session.Resize(msg.Cols, msg.Rows)
					} else {
						// 会话尚未准备好，记录最新的 resize 请求
						pendingResize = &struct{ cols, rows int }{cols: msg.Cols, rows: msg.Rows}
					}
					swMu.Unlock()
				}
				return nil
			case "ping":
				return nil // 忽略心跳消息
			case "input":
				data = []byte(msg.Data) // 使用嵌套的数据作为输入。
			default:
				return nil // 忽略其他未知类型的 JSON 消息
			}
		}

		// 获取当前的 SSH 会话引用
		swMu.Lock()
		defer swMu.Unlock()

		if sw == nil {
			// 会话尚未准备好，缓冲输入
			pendingInput = append(pendingInput, data...)
			return nil
		}

		// 将原始数据（或提取的输入数据）传递给 SSH 会话。
		sw.mu.Lock()
		defer sw.mu.Unlock()
		if !sw.closed && sw.stdinWriter != nil {
			if _, err := sw.stdinWriter.Write(data); err != nil {
				return err // 传播错误以关闭连接。
			}
			sw.LastActive = time.Now()
		}
		return nil
	}

	// 为 websocket 客户端创建元数据。
	metadata := &websocket.SessionMetadata{UserID: userID, SessionType: "terminal", Role: "user"}

	// 1. 先将客户端添加到池中。
	wsClient := s.wsPool.Add(userID, conn, metadata, messageHandler)

	// 2. 创建新的 SSH 会话（一对一绑定）。
	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		// 这里我们无法直接关闭 conn，因为 wsPool 已经接管了
		// 但 wsPool.Add 会启动读写 pump，如果 wsClient 被关闭，conn 也会被关闭
		wsClient.Close()
		return err
	}

	// 创建专用的 Writer
	writer := &PrivateTerminalWriter{client: wsClient}

	// 创建新的终端会话
	session, err := sshClient.NewTerminalSession(writer, writer, cols, rows)
	if err != nil {
		wsClient.Close()
		return err
	}

	swInstance := &sshSessionWrapper{
		ID:          fmt.Sprintf("%d:%v:%d", userID, isRoot, time.Now().UnixNano()), // 唯一ID
		Session:     session,
		stdinWriter: session.Stdin,
		LastActive:  time.Now(),
	}

	// 3. 更新会话引用
	swMu.Lock()
	sw = swInstance
	// 将缓冲的输入写入会话
	if len(pendingInput) > 0 {
		sw.mu.Lock()
		if !sw.closed && sw.stdinWriter != nil {
			_, _ = sw.stdinWriter.Write(pendingInput)
		}
		sw.mu.Unlock()
		pendingInput = nil
	}
	// 应用缓冲的 resize 请求
	if pendingResize != nil {
		_ = sw.Session.Resize(pendingResize.cols, pendingResize.rows)
		pendingResize = nil
	}
	swMu.Unlock()

	// 4. 生命周期管理
	// 当 SSH 会话结束时，关闭 WebSocket
	go func() {
		_ = session.Session.Wait()
		swInstance.close()
		wsClient.Close()
	}()

	// 当 WebSocket 结束时，关闭 SSH 会话
	go func() {
		<-wsClient.Done
		swInstance.close()
	}()

	return nil
}

// ResizeTerminal 调整终端大小
func (s *terminalService) ResizeTerminal(userID int, isRoot bool, cols, rows int) error {
	key := fmt.Sprintf("%d:%v", userID, isRoot)
	s.mu.RLock()
	sw, exists := s.sessions[key]
	s.mu.RUnlock()

	if !exists {
		return pkgerrors.New(pkgerrors.CodeNotFound, "终端会话不存在")
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.LastActive = time.Now()

	if sw.Session != nil {
		return sw.Session.Resize(cols, rows)
	}
	return nil
}

// closeSshSession 关闭底层 SSH 会话并清理资源。
func (s *terminalService) closeSshSession(userID int, isRoot bool) {
	key := fmt.Sprintf("%d:%v", userID, isRoot)
	s.mu.Lock()
	sw, exists := s.sessions[key]
	if exists {
		delete(s.sessions, key)
	}
	s.mu.Unlock()

	if !exists {
		return
	}

	sw.close()
}

// cleanupIdleSessions 定期清理过期的 SSH 会话。
func (s *terminalService) cleanupIdleSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	timeout := 30 * time.Minute // 30分钟无活动则关闭 SSH 会话

	for range ticker.C {
		var keysToClose []string
		s.mu.RLock()
		for key, sw := range s.sessions {
			sw.mu.Lock()
			if time.Since(sw.LastActive) > timeout {
				keysToClose = append(keysToClose, key)
			}
			sw.mu.Unlock()
		}
		s.mu.RUnlock()

		for _, key := range keysToClose {
			s.mu.Lock()
			sw, exists := s.sessions[key]
			if exists {
				delete(s.sessions, key)
				sw.close() // 使用集中式关闭方法
				zap.L().Info("清理空闲的 SSH 会话", zap.String("id", key))
			}
			s.mu.Unlock()
		}
	}
}

// close 是安全关闭会话资源的辅助方法。
func (sw *sshSessionWrapper) close() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.closed {
		sw.closed = true
		if sw.Session != nil {
			_ = sw.Session.Close()
		}
		if sw.stdinWriter != nil {
			_ = sw.stdinWriter.Close()
		}
		zap.L().Info("已关闭 SSH 会话", zap.String("id", sw.ID))
	}
}
