package websocket

import (
	"cloudque/pkg/logger"
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
	SessionType string // "terminal" or "systemInfo"
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

// ConnectionPool WebSocket 连接池
type ConnectionPool struct {
	// userID -> { client -> struct{} }
	userClients map[int]map[*Client]struct{}
	// 管理员客户端
	adminClients map[*Client]struct{}
	// systemInfo 类型的连接计数（优化频繁检查）
	systemInfoCount int
	mu              sync.RWMutex
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
	// 如果是管理员，添加到管理员客户端 map
	if metadata != nil && metadata.Role == "admin" {
		p.adminClients[client] = struct{}{}
		// 如果是 systemInfo 类型，增加计数
		if metadata.SessionType == "systemInfo" {
			p.systemInfoCount++
		}
	} else {
		if p.userClients[userID] == nil {
			p.userClients[userID] = make(map[*Client]struct{})
		}
		p.userClients[userID][client] = struct{}{}
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

		if c.Metadata.Role == "user" {
			userID := c.Metadata.UserID
			//关闭该 userID 的一个客户端连接
			if clients, ok := c.Pool.userClients[userID]; ok {
				delete(clients, c)
				if len(clients) == 0 {
					delete(c.Pool.userClients, userID)
				}
			}
		} else if c.Metadata.Role == "admin" {
			// 如果是管理员，从管理员客户端 map 中移除
			delete(c.Pool.adminClients, c)
			// 如果是 systemInfo 类型，减少计数
			if c.Metadata.SessionType == "systemInfo" {
				if c.Pool.systemInfoCount > 0 {
					c.Pool.systemInfoCount--
				}
			}
		}
		close(c.Done)
		close(c.Send)
		err := c.Conn.Close()
		if err != nil {
			logger.Errorf("用户 id 为%v的客户端关闭失败%v", c.Metadata.UserID, err)
			return
		}
	})
}

// ReadPump 负责从WebSocket连接读取消息（主要用于处理Pong和Close消息）
func (c *Client) ReadPump() {
	defer func() {
		c.Close()
	}()

	c.Conn.SetReadLimit(1024) // 设置最大读取消息大小，防止恶意大包
	if err := c.Conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		logger.Errorf("设置超时时间出错%v", err)
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	for {
		messageType, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("websocket读协程出错%v", err)
			}
			break
		}

		// 刷新读取超时
		err = c.Conn.SetReadDeadline(time.Now().Add(PongWait))
		if err != nil {
			logger.Errorf("刷新读取超时导致出错%v", err)
			break
		}

		// 如果定义了消息处理回调，则调用它
		if c.MessageHandler != nil {
			if err = c.MessageHandler(c, messageType, data); err != nil {
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
				err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					logger.Errorf("写入失败，连接已断开%v", err)
				}
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logger.Errorf("写入失败%v", err)
				return
			}
			_, err = w.Write(message)
			if err != nil {
				logger.Errorf("写入失败%v", err)
				return
			}

			if err = w.Close(); err != nil {
				logger.Errorf("关闭写入失败%v", err)
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

// BroadcastToAdminsByType 广播消息给指定类型的用户
func (p *ConnectionPool) BroadcastToAdminsByType(sessionType string, data []byte) {
	p.mu.RLock()
	// 先收集需要发送的客户端，避免在持有锁时执行耗时操作
	var clientsToSend []*Client
	for client := range p.adminClients {
		if client.Metadata != nil && client.Metadata.SessionType == sessionType {
			clientsToSend = append(clientsToSend, client)
		}
	}
	p.mu.RUnlock()

	// 在锁外发送消息，避免死锁
	for _, client := range clientsToSend {
		select {
		case client.Send <- data:
		default:
			// 如果发送通道满了，说明客户端消费慢，记录日志并跳过
			// 不立即关闭连接，避免频繁断开
			logger.Warnf("客户端发送通道已满，跳过消息发送，用户 ID: %d", client.Metadata.UserID)
		}
	}
}

// IsHavingSystemInfoConnection 判断是否有获取系统消息 ws 连接
// 使用计数优化，避免每次遍历所有连接
func (p *ConnectionPool) IsHavingSystemInfoConnection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.systemInfoCount > 0
}

// GetSystemInfoConnectionCount 获取 systemInfo 类型的连接数（用于调试和监控）
func (p *ConnectionPool) GetSystemInfoConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.systemInfoCount
}

// GetTotalConnectionCount 获取总连接数（用于调试和监控）
func (p *ConnectionPool) GetTotalConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := len(p.adminClients)
	for _, clients := range p.userClients {
		total += len(clients)
	}
	return total
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
	// 同时收集 adminClients
	var allAdminClients []*Client
	for client := range p.adminClients {
		allAdminClients = append(allAdminClients, client)
	}
	p.mu.RUnlock()

	// 关闭所有用户连接
	for _, client := range allClients {
		client.Close()
	}
	// 关闭所有管理员连接
	for _, client := range allAdminClients {
		client.Close()
	}
}
