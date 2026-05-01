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
)

// 终端输出缓存
const terminalOutputBufferLimit = 2 << 20 // 2 MB

// sshSessionWrapper 保存活动的 SSH PTY 会话、在线 WebSocket 和最近输出。
type sshSessionWrapper struct {
	ID            string
	Key           string
	Session       *server.TerminalSession
	stdinWriter   io.WriteCloser
	LastActive    time.Time
	clients       map[*websocket.Client]struct{}
	outputBuffer  []byte
	utf8Remainder []byte
	maxBufferSize int
	mu            sync.Mutex
	closed        bool
}

// terminalService 终端服务实现
type terminalService struct {
	sessionManager *ssh.SessionManager
	authService    AuthService
	wsPool         *websocket.ConnectionPool
	sessions       map[string]*sshSessionWrapper
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
	go ts.cleanupIdleSessions()
	return ts
}

// TerminalBroadcastWriter 将 SSH 输出写入会话缓存，并广播给当前在线客户端。
type TerminalBroadcastWriter struct {
	session *sshSessionWrapper
}

// Write 将 SSH PTY 输出追加到缓存并广播。
func (w *TerminalBroadcastWriter) Write(p []byte) (int, error) {
	if w == nil || w.session == nil {
		return len(p), nil
	}

	w.session.mu.Lock()
	data := append(w.session.utf8Remainder, p...)
	w.session.utf8Remainder = nil

	n := len(data)
	cut := n
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

	toSend := data[:cut]
	w.session.utf8Remainder = data[cut:]
	if len(toSend) == 0 {
		w.session.mu.Unlock()
		return len(p), nil
	}

	msg := terminalOutputMessage(toSend)
	w.session.appendOutputLocked(toSend)
	clients := w.session.clientListLocked()
	w.session.mu.Unlock()

	for _, client := range clients {
		safeSendTerminalMessage(client, msg)
	}

	return len(p), nil
}

func terminalOutputMessage(data []byte) []byte {
	msg := struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}{
		Type: "output",
		Data: string(data),
	}
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		logger.Errorf("序列化终端输出失败: %v", err)
		return nil
	}
	return jsonBytes
}

func safeSendTerminalMessage(client *websocket.Client, msg []byte) {
	if client == nil || len(msg) == 0 {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("发送终端消息失败，客户端通道已关闭: %v", r)
		}
	}()
	select {
	case client.Send <- msg:
	default:
		logger.Warnf("终端客户端发送缓冲区已满: userID=%d", client.Metadata.UserID)
	}
}

func (sw *sshSessionWrapper) appendOutputLocked(data []byte) {
	sw.outputBuffer = append(sw.outputBuffer, data...)
	if sw.maxBufferSize <= 0 {
		sw.maxBufferSize = terminalOutputBufferLimit
	}
	if len(sw.outputBuffer) > sw.maxBufferSize {
		sw.outputBuffer = append([]byte(nil), sw.outputBuffer[len(sw.outputBuffer)-sw.maxBufferSize:]...)
	}
	sw.LastActive = time.Now()
}

func (sw *sshSessionWrapper) addClient(client *websocket.Client) []byte {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.closed {
		return nil
	}
	if sw.clients == nil {
		sw.clients = make(map[*websocket.Client]struct{})
	}
	sw.clients[client] = struct{}{}
	sw.LastActive = time.Now()
	return append([]byte(nil), sw.outputBuffer...)
}

func (sw *sshSessionWrapper) removeClient(client *websocket.Client) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.clients, client)
	sw.LastActive = time.Now()
}

func (sw *sshSessionWrapper) closeAllClients() {
	sw.mu.Lock()
	if sw.closed {
		sw.mu.Unlock()
		return
	}
	clients := sw.clientListLocked()
	sw.clients = make(map[*websocket.Client]struct{})
	sw.mu.Unlock()
	for _, client := range clients {
		client.Close()
	}
}

func (sw *sshSessionWrapper) clientListLocked() []*websocket.Client {
	clients := make([]*websocket.Client, 0, len(sw.clients))
	for client := range sw.clients {
		clients = append(clients, client)
	}
	return clients
}

func (sw *sshSessionWrapper) resize(cols, rows int) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.closed || sw.Session == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "终端会话不存在")
	}
	sw.LastActive = time.Now()
	return sw.Session.Resize(cols, rows)
}

func (sw *sshSessionWrapper) writeInput(data []byte) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.closed || sw.stdinWriter == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "终端会话不存在")
	}
	if _, err := sw.stdinWriter.Write(data); err != nil {
		return err
	}
	sw.LastActive = time.Now()
	return nil
}

// close 是安全关闭会话资源的辅助方法。closeClients=true 时会关闭所有绑定 WebSocket。
func (sw *sshSessionWrapper) close(closeClients bool) {
	sw.mu.Lock()
	if sw.closed {
		sw.mu.Unlock()
		return
	}
	sw.closed = true

	clients := sw.clientListLocked()
	sw.clients = make(map[*websocket.Client]struct{})
	sw.outputBuffer = nil
	sw.utf8Remainder = nil

	session := sw.Session
	stdin := sw.stdinWriter
	sw.Session = nil
	sw.stdinWriter = nil
	sw.mu.Unlock()

	if session != nil {
		if err := session.Close(); err != nil {
			logger.Errorf("关闭SSH终端会话失败: id=%s, err=%v", sw.ID, err)
		}
	}
	if stdin != nil {
		if err := stdin.Close(); err != nil {
			logger.Errorf("关闭SSH标准输入失败: id=%s, err=%v", sw.ID, err)
		}
	}
	if closeClients {
		for _, client := range clients {
			client.Close()
		}
	}
	logger.Infof("已关闭 SSH 终端会话: id=%s", sw.ID)
}

func terminalSessionKey(userID int, isRoot bool) string {
	return fmt.Sprintf("%d:%v", userID, isRoot)
}

// getSSHClient 获取用户的SSH客户端
func (s *terminalService) getSSHClient(userID int, isRoot bool) (*server.Client, error) {
	if s.sessionManager == nil {
		logger.Error("SSH会话管理器未初始化")
		return nil, pkgerrors.New(pkgerrors.CodeInternalError, "SSH会话管理器未初始化")
	}

	session, err := s.sessionManager.GetSession(userID, isRoot)
	if err == nil && session != nil && session.Client != nil {
		if _, err := session.Client.ExecuteCommand("echo 1"); err == nil {
			return session.Client, nil
		}
	}

	if err := s.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
		logger.Errorf("恢复SSH会话失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "恢复SSH会话失败", err)
	}

	session, err = s.sessionManager.GetSession(userID, isRoot)
	if err != nil {
		logger.Errorf("恢复后获取SSH会话失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "获取SSH会话失败", err)
	}
	if session == nil || session.Client == nil {
		logger.Errorf("SSH会话获取成功但客户端为空: userID=%d, isRoot=%t", userID, isRoot)
		return nil, pkgerrors.New(pkgerrors.CodeInternalError, "SSH客户端不可用")
	}
	return session.Client, nil
}

func (s *terminalService) getOrCreateTerminalSession(userID int, isRoot bool, cols, rows int) (*sshSessionWrapper, error) {
	key := terminalSessionKey(userID, isRoot)

	s.mu.Lock()
	defer s.mu.Unlock()

	if sw, ok := s.sessions[key]; ok {
		sw.mu.Lock()
		closed := sw.closed
		sw.mu.Unlock()
		if !closed {
			logger.Infof("getOrCreateTerminalSession: 复用已有终端会话 userID=%d, isRoot=%t, sessionID=%s", userID, isRoot, sw.ID)
			return sw, nil
		}
		delete(s.sessions, key)
		logger.Infof("getOrCreateTerminalSession: 旧会话已关闭，删除 userID=%d, isRoot=%t", userID, isRoot)
	}

	sshClient, err := s.getSSHClient(userID, isRoot)
	if err != nil {
		logger.Errorf("获取SSH客户端失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, err
	}

	sw := &sshSessionWrapper{
		ID:            fmt.Sprintf("%d:%v:%d", userID, isRoot, time.Now().UnixNano()),
		Key:           key,
		LastActive:    time.Now(),
		clients:       make(map[*websocket.Client]struct{}),
		maxBufferSize: terminalOutputBufferLimit,
	}
	writer := &TerminalBroadcastWriter{session: sw}

	session, err := sshClient.NewTerminalSession(writer, writer, cols, rows)
	if err != nil {
		logger.Errorf("创建终端会话失败: userID=%d, isRoot=%t, err=%v", userID, isRoot, err)
		return nil, err
	}

	sw.Session = session
	sw.stdinWriter = session.Stdin
	s.sessions[key] = sw
	logger.Infof("getOrCreateTerminalSession: 创建新终端会话 userID=%d, isRoot=%t, sessionID=%s", userID, isRoot, sw.ID)

	go func() {
		_ = session.Session.Wait()
		s.closeTerminalSessionByKey(key, sw, true)
	}()

	return sw, nil
}

// HandleTerminalConnection 处理新的 websocket 连接，并将其绑定到用户级 SSH PTY 会话。
func (s *terminalService) HandleTerminalConnection(userID int, isRoot bool, cols, rows int, conn *gwebsocket.Conn) error {
	if err := s.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "无法建立SSH会话", err)
	}

	s.closeTerminalSession(userID, isRoot, true)

	var sw *sshSessionWrapper
	var pendingInput []byte
	var pendingResize *struct{ cols, rows int }
	var initMu sync.Mutex

	messageHandler := func(client *websocket.Client, messageType int, data []byte) error {
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
					initMu.Lock()
					if sw == nil {
						pendingResize = &struct{ cols, rows int }{cols: msg.Cols, rows: msg.Rows}
						initMu.Unlock()
						return nil
					}
					current := sw
					initMu.Unlock()
					if err := current.resize(msg.Cols, msg.Rows); err != nil {
						logger.Errorf("调整终端大小失败: userID=%d, cols=%d, rows=%d, err=%v", userID, msg.Cols, msg.Rows, err)
					}
				}
				return nil
			case "ping":
				return nil
			case "close":
				s.closeTerminalSession(userID, isRoot, false)
				return fmt.Errorf("terminal closed by user")
			case "input":
				data = []byte(msg.Data)
			default:
				return nil
			}
		}

		initMu.Lock()
		if sw == nil {
			pendingInput = append(pendingInput, data...)
			initMu.Unlock()
			return nil
		}
		current := sw
		initMu.Unlock()

		if err := current.writeInput(data); err != nil {
			logger.Errorf("写入SSH标准输入失败: userID=%d, err=%v", userID, err)
			return err
		}
		return nil
	}

	metadata := &websocket.SessionMetadata{UserID: userID, SessionType: "terminal", Role: "user"}
	wsClient := s.wsPool.Add(userID, conn, metadata, messageHandler)

	swInstance, err := s.getOrCreateTerminalSession(userID, isRoot, cols, rows)
	if err != nil {
		wsClient.Close()
		return err
	}

	initMu.Lock()
	sw = swInstance
	if pendingResize != nil {
		if err := sw.resize(pendingResize.cols, pendingResize.rows); err != nil {
			logger.Errorf("应用缓冲resize请求失败: userID=%d, cols=%d, rows=%d, err=%v", userID, pendingResize.cols, pendingResize.rows, err)
		}
		pendingResize = nil
	}
	if len(pendingInput) > 0 {
		if err := sw.writeInput(pendingInput); err != nil {
			logger.Errorf("写入缓冲输入到SSH失败: userID=%d, err=%v", userID, err)
		}
		pendingInput = nil
	}
	initMu.Unlock()

	sw.addClient(wsClient)
	safeSendTerminalMessage(wsClient, terminalOutputMessage([]byte("\x1b[2J\x1b[H")))

	go func() {
		<-wsClient.Done
		sw.removeClient(wsClient)
		logger.Infof("终端 WebSocket 已断开: userID=%d, isRoot=%t", userID, isRoot)
	}()

	return nil
}

// ResizeTerminal 调整终端大小
func (s *terminalService) ResizeTerminal(userID int, isRoot bool, cols, rows int) error {
	key := terminalSessionKey(userID, isRoot)
	s.mu.RLock()
	sw, exists := s.sessions[key]
	s.mu.RUnlock()
	if !exists {
		logger.Warnf("终端会话不存在，无法调整大小: userID=%d, isRoot=%t", userID, isRoot)
		return pkgerrors.New(pkgerrors.CodeNotFound, "终端会话不存在")
	}
	return sw.resize(cols, rows)
}

func (s *terminalService) closeTerminalSession(userID int, isRoot bool, closeClients bool) {
	key := terminalSessionKey(userID, isRoot)
	s.mu.Lock()
	sw, exists := s.sessions[key]
	if exists {
		delete(s.sessions, key)
		logger.Infof("closeTerminalSession: 删除旧终端会话 userID=%d, isRoot=%t, sessionID=%s", userID, isRoot, sw.ID)
	} else {
		logger.Infof("closeTerminalSession: 无旧终端会话 userID=%d, isRoot=%t", userID, isRoot)
	}
	s.mu.Unlock()
	if exists {
		sw.close(closeClients)
	}
}

func (s *terminalService) closeTerminalSessionByKey(key string, expected *sshSessionWrapper, closeClients bool) {
	s.mu.Lock()
	sw, exists := s.sessions[key]
	if exists && sw == expected {
		delete(s.sessions, key)
	} else if exists {
		sw = nil
	}
	s.mu.Unlock()
	if sw != nil {
		sw.close(closeClients)
	}
}

// CloseUserTerminals 关闭某个用户的所有终端 PTY 和输出缓存。
func (s *terminalService) CloseUserTerminals(userID int) {
	prefixRoot := fmt.Sprintf("%d:", userID)
	var sessionsToClose []*sshSessionWrapper

	s.mu.Lock()
	for key, sw := range s.sessions {
		if len(key) >= len(prefixRoot) && key[:len(prefixRoot)] == prefixRoot {
			delete(s.sessions, key)
			sessionsToClose = append(sessionsToClose, sw)
		}
	}
	s.mu.Unlock()

	for _, sw := range sessionsToClose {
		sw.close(true)
	}
}

// CloseAllTerminals 关闭所有终端 PTY 和输出缓存。
func (s *terminalService) CloseAllTerminals() {
	var sessionsToClose []*sshSessionWrapper

	s.mu.Lock()
	for key, sw := range s.sessions {
		delete(s.sessions, key)
		sessionsToClose = append(sessionsToClose, sw)
	}
	s.mu.Unlock()

	for _, sw := range sessionsToClose {
		sw.close(true)
	}
}

// cleanupIdleSessions 定期清理过期的 SSH PTY 会话。
func (s *terminalService) cleanupIdleSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	timeout := 30 * time.Minute

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
			}
			s.mu.Unlock()
			if exists {
				sw.close(true)
				logger.Infof("清理空闲的 SSH 终端会话: id=%s", key)
			}
		}
	}
}
