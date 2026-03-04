package websocket

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// WriteWait 写入超时时间
	WriteWait = 10 * time.Second
	// PongWait 等待 Pong 的时间
	PongWait = 60 * time.Second
	// PingPeriod 发送 Ping 的周期 (必须小于 PongWait)
	PingPeriod = (PongWait * 9) / 10
)

// SessionMetadata 会话元数据
type SessionMetadata struct {
	UserID      int
	SessionType string // "terminal" or "files" or "ws"
	Role        string // "admin" or "user"
	CreatedAt   int64
}

// Client 包装了 WebSocket 连接和发送缓冲区
type Client struct {
	Pool     *ConnectionPool
	Conn     *websocket.Conn
	Send     chan []byte
	Metadata *SessionMetadata
	once     sync.Once
}

// ConnectionPool WebSocket连接池（优化版：读写分离、心跳检测）
type ConnectionPool struct {
	// userID -> { client -> struct{} }
	userClients map[int]map[*Client]struct{}
	// 管理员客户端
	adminClients map[*Client]struct{}
	mu           sync.RWMutex
}

// NewConnectionPool 创建连接池
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		userClients:  make(map[int]map[*Client]struct{}),
		adminClients: make(map[*Client]struct{}),
	}
}

// Add 创建并添加一个新客户端
func (p *ConnectionPool) Add(userID int, conn *websocket.Conn, metadata *SessionMetadata) *Client {
	client := &Client{
		Pool:     p,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Metadata: metadata,
	}

	p.mu.Lock()
	if p.userClients[userID] == nil {
		p.userClients[userID] = make(map[*Client]struct{})
	}
	p.userClients[userID][client] = struct{}{}

	// 如果是管理员，添加到管理员客户端map
	if metadata != nil && metadata.Role == "admin" {
		p.adminClients[client] = struct{}{}
	}
	p.mu.Unlock()

	// 启动写协程
	go client.WritePump()
	// 启动读协程处理控制消息（Ping/Pong/Close）
	go client.ReadPump()

	return client
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.once.Do(func() {
		c.Pool.mu.Lock()
		defer c.Pool.mu.Unlock()

		userID := c.Metadata.UserID
		if clients, ok := c.Pool.userClients[userID]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(c.Pool.userClients, userID)
			}
		}

		// 如果是管理员，从管理员客户端map中移除
		delete(c.Pool.adminClients, c)

		close(c.Send)
		_ = c.Conn.Close()
	})
}

// ReadPump 负责从WebSocket连接读取消息（主要用于处理Pong和Close消息）
func (c *Client) ReadPump() {
	defer func() {
		c.Close()
	}()

	c.Conn.SetReadLimit(512) // 设置最大读取消息大小，防止恶意大包
	if err := c.Conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// log error if needed
			}
			break
		}
		// 丢弃读取到的消息，因为目前只需要单向推送
	}
}

// WritePump 负责将消息从通道写入网络（每个连接一个协程）
func (c *Client) WritePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				return
			}
			if !ok {
				// 通道已关闭
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, err = w.Write(message)
			if err != nil {
				return
			}

			// 将队列中剩余的消息合并发送
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, err2 := w.Write([]byte("\n"))
				if err2 != nil {
					return
				}
				_, err3 := w.Write(<-c.Send)
				if err3 != nil {
					return
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				return
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendToUser 发送消息给指定用户的所有客户端
func (p *ConnectionPool) SendToUser(userID int, data []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if clients, ok := p.userClients[userID]; ok {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				// 如果发送缓冲区满了，主动关闭这个慢连接，防止阻塞整个系统
				go client.Close()
			}
		}
	}
}

// Broadcast 广播消息给所有用户
func (p *ConnectionPool) Broadcast(data []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, clients := range p.userClients {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				go client.Close()
			}
		}
	}
}

// BroadcastToAdmins 广播消息给管理员用户
func (p *ConnectionPool) BroadcastToAdmins(data []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for client := range p.adminClients {
		select {
		case client.Send <- data:
		default:
			go client.Close()
		}
	}
}

// GetConnectionCount 获取总连接数
func (p *ConnectionPool) GetConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, clients := range p.userClients {
		count += len(clients)
	}
	return count
}

// GetAdminConnectionCount 获取管理员连接数
func (p *ConnectionPool) GetAdminConnectionCount() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for client, _ := range p.adminClients {
		if client.Metadata.SessionType == "ws" {
			return true
		}
	}
	return false
}

// CloseAll 关闭所有连接
func (p *ConnectionPool) CloseAll() {
	p.mu.RLock()
	// 收集所有 client 避免在循环中持有写锁导致 Close() 死锁
	var allClients []*Client
	for _, clients := range p.userClients {
		for client := range clients {
			allClients = append(allClients, client)
		}
	}
	p.mu.RUnlock()

	for _, client := range allClients {
		client.Close()
	}
}
