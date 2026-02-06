package websocket

import (
	"sync"

	"github.com/gorilla/websocket"
)

// SessionMetadata 会话元数据
type SessionMetadata struct {
	UserID      uint
	SessionType string // "terminal" or "files"
	CreatedAt   int64
}

// ConnectionPool WebSocket连接池（增强版：与SSH会话关联）
type ConnectionPool struct {
	connections map[uint]*websocket.Conn
	metadata    map[uint]*SessionMetadata
	mu          sync.RWMutex
}

// NewConnectionPool 创建连接池
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[uint]*websocket.Conn),
		metadata:    make(map[uint]*SessionMetadata),
	}
}

// Add 添加连接
func (p *ConnectionPool) Add(userID uint, conn *websocket.Conn, metadata *SessionMetadata) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 如果用户已有连接，先关闭旧连接（或者支持多连接，这里简化为单连接）
	if old, ok := p.connections[userID]; ok {
		_ = old.Close()
		delete(p.metadata, userID)
	}
	p.connections[userID] = conn
	if metadata != nil {
		p.metadata[userID] = metadata
	}
}

// Remove 移除连接
func (p *ConnectionPool) Remove(userID uint) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conn, ok := p.connections[userID]; ok {
		_ = conn.Close()
		delete(p.connections, userID)
		delete(p.metadata, userID)
	}
}

// Get 获取连接
func (p *ConnectionPool) Get(userID uint) *websocket.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connections[userID]
}

// GetMetadata 获取元数据
func (p *ConnectionPool) GetMetadata(userID uint) *SessionMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata[userID]
}

// Broadcast 广播消息
func (p *ConnectionPool) Broadcast(message []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, conn := range p.connections {
		_ = conn.WriteMessage(websocket.TextMessage, message)
	}
}

// GetAllUserIDs 获取所有用户ID
func (p *ConnectionPool) GetAllUserIDs() []uint {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]uint, 0, len(p.connections))
	for userID := range p.connections {
		ids = append(ids, userID)
	}
	return ids
}

// GetConnectionCount 获取连接数
func (p *ConnectionPool) GetConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// CloseAll 关闭所有连接
func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for userID, conn := range p.connections {
		_ = conn.Close()
		delete(p.connections, userID)
		delete(p.metadata, userID)
	}
}
