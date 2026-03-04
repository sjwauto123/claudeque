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
	SessionType string // "terminal" or "systeminfo"
	Role        string // "admin" or "user"
	CreatedAt   int64
}

// MessageHandler 处理接收到的消息
type MessageHandler func(client *Client, messageType int, data []byte) error

// Client 包装了 WebSocket 连接和发送缓冲区
type Client struct {
	Pool           *ConnectionPool // 连接池引用,该客户端所属的连接池
	Conn           *websocket.Conn // WebSocket 连接
	Send           chan []byte     // 发送缓冲区
	Metadata       *SessionMetadata
	MessageHandler MessageHandler // 消息处理回调
	Done           chan struct{}  // 用于通知连接已关闭
	once           sync.Once      // 确保关闭连接只执行一次
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
func (p *ConnectionPool) Add(userID int, conn *websocket.Conn, metadata *SessionMetadata, handler MessageHandler) *Client {

	client := &Client{
		Pool:           p,
		Conn:           conn,
		Send:           make(chan []byte, 1024),
		Metadata:       metadata,
		MessageHandler: handler,
		Done:           make(chan struct{}),
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
		//关闭该userID的一个客户端连接
		if clients, ok := c.Pool.userClients[userID]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(c.Pool.userClients, userID)
			}
		}

		// 如果是管理员，从管理员客户端map中移除
		delete(c.Pool.adminClients, c)

		close(c.Done)
		close(c.Send)
		_ = c.Conn.Close()
	})
}

// ReadPump 负责从WebSocket连接读取消息（主要用于处理Pong和Close消息）
func (c *Client) ReadPump() {
	defer func() {
		c.Close()
	}()

	c.Conn.SetReadLimit(1024) // 设置最大读取消息大小，防止恶意大包
	if err := c.Conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	for {
		messageType, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// log error if needed
			}
			break
		}

		// 刷新读取超时
		_ = c.Conn.SetReadDeadline(time.Now().Add(PongWait))

		// 如果定义了消息处理回调，则调用它
		if c.MessageHandler != nil {
			if err := c.MessageHandler(c, messageType, data); err != nil {
				break
			}
		}
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

// SendToUserByType 发送消息给指定用户且指定会话类型的客户端
func (p *ConnectionPool) SendToUserByType(userID int, sessionType string, data []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if clients, ok := p.userClients[userID]; ok {
		for client := range clients {
			// 过滤 SessionType
			if client.Metadata != nil && client.Metadata.SessionType == sessionType {
				select {
				case client.Send <- data:
				default:
					// 如果发送缓冲区满了，主动关闭这个慢连接
					go client.Close()
				}
			}
		}
	}
}

// BroadcastToAdminsByType 广播消息给指定类型的管理员用户
func (p *ConnectionPool) BroadcastToAdminsByType(sessionType string, data []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for client := range p.adminClients {
		if client.Metadata != nil && client.Metadata.SessionType == sessionType {
			select {
			case client.Send <- data:
			default:
				go client.Close()
			}
		}
	}
}

// GetAdminConnectionCount 获取管理员连接数
func (p *ConnectionPool) GetAdminConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.adminClients)
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
