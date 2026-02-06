// internal/ws/hub.go

package ws

import (
	"log"
	"sync"
	"time"
)

type Hub struct {
	clients      map[*Client]bool
	UserClients  map[string]map[*Client]bool // userID -> clients
	AdminClients map[*Client]bool            // 可选，如果你还需要 admin 集合

	Register     chan *Client
	Unregister   chan *Client
	Done         chan struct{}
	StartCollect chan bool
	StopCollect  chan bool
	mutex        sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		UserClients:  make(map[string]map[*Client]bool),
		AdminClients: make(map[*Client]bool),
		Register:     make(chan *Client, 100),
		Unregister:   make(chan *Client, 100),
		Done:         make(chan struct{}),
		StartCollect: make(chan bool),
		StopCollect:  make(chan bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mutex.Lock()
			// 记录之前的客户端数量
			prevCount := len(h.clients)

			h.clients[client] = true

			// 初始化 UserClients[user]
			if _, ok := h.UserClients[client.UserID]; !ok {
				h.UserClients[client.UserID] = make(map[*Client]bool)
			}
			h.UserClients[client.UserID][client] = true

			// 标记为管理员客户端
			h.AdminClients[client] = true

			// 检查是否需要启动收集器
			newCount := len(h.clients)
			if prevCount == 0 && newCount > 0 {
				// 第一个客户端连接，启动收集器
				h.StartCollect <- true
			}

			h.mutex.Unlock()

		case client := <-h.Unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				// 记录之前的客户端数量
				prevCount := len(h.clients)

				delete(h.clients, client)
				delete(h.AdminClients, client)

				if userMap, exists := h.UserClients[client.UserID]; exists {
					delete(userMap, client)
					if len(userMap) == 0 {
						delete(h.UserClients, client.UserID)
					}
				}
				close(client.Send)

				// 检查是否需要停止收集器
				newCount := len(h.clients)
				if prevCount > 0 && newCount == 0 {
					// 最后一个客户端断开，停止收集器
					h.StopCollect <- true
				}
			}
			h.mutex.Unlock()
		}
	}
}

// SendToClient 安全地向单个 client 发送消息
func (h *Hub) SendToClient(client *Client, data []byte) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// 检查 client 是否仍注册（未断开）
	if !h.clients[client] {
		return false
	}

	select {
	case client.Send <- data:
		return true
	case <-time.After(100 * time.Millisecond): // 短超时
		log.Printf("向连接[%s]发送消息超时（缓冲区满）", client.UserID)
		// 可选：降级策略（如丢弃/入消息队列）
		return false
	default:
		// 缓冲区满，丢弃（或可记录日志）
		return false
	}
}

// SendToAllAdmins 向所有管理员客户端发送消息
func (h *Hub) SendToAllAdmins(data []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.AdminClients {
		if h.clients[client] {
			select {
			case client.Send <- data:
				// 发送成功
			case <-time.After(100 * time.Millisecond):
				log.Printf("向管理员连接[%s]发送消息超时（缓冲区满）", client.UserID)
			default:
				// 缓冲区满，丢弃
			}
		}
	}
}
