package terminal

import (
	"cloudque/pkg/logger"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"cloudque/internal/middleware"
	"cloudque/internal/service"
	"cloudque/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Controller 终端控制器
type Controller struct {
	terminalService service.TerminalService
	authService     service.AuthService
	userLogService  service.UserOperationLogService
}

// NewController 创建终端控制器
func NewController(terminalService service.TerminalService, authService service.AuthService, userLogService service.UserOperationLogService) *Controller {
	return &Controller{
		terminalService: terminalService,
		authService:     authService,
		userLogService:  userLogService,
	}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WebSocketTerminal WebSocket 终端透传：将浏览器与 SSH 终端双向透传
// 认证：Query 参数 token=JWT 或 Header Authorization: Bearer <token>
func (ctrl *Controller) WebSocketTerminal(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	// 判断身份：根据用户权限决定是 root 终端还是普通用户终端
	// 如果用户拥有系统级权限，则使用 root 终端，否则使用普通用户终端
	isRoot, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeTerminal)
	if err != nil {
		response.BizError(c, err)
		return
	}

	// 确保对应身份的 SSH 会话存在
	// 注意：如果 Redis 中没有凭证（例如用户很久没登录），这里会失败，提示用户重新登录
	if err := ctrl.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
		logger.Error("无法建立SSH会话", zap.Error(err))
		response.BizError(c, err)
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Error("无法关闭web连接")
		}
	}(conn)

	// 设置心跳和超时
	const pongWait = 60 * time.Second
	err = conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return
	}
	conn.SetPongHandler(func(string) error {
		err := conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return nil
	})

	// 获取或创建持久化终端会话
	// 默认大小 80x24，后续可通过 resize 消息调整
	pt, err := ctrl.terminalService.GetOrCreateTerminal(userID, isRoot, 80, 24)
	if err != nil {
		response.BizError(c, err)
		return
	}
	// 注意：不要在 defer 中关闭 Session，因为它是持久化的

	type inboundMsg struct {
		Type string `json:"type"`
		Data string `json:"data"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}

	writeCh := make(chan []byte, 64)
	var writeWg sync.WaitGroup
	writeWg.Add(1)
	go func() {
		defer writeWg.Done()
		for msg := range writeCh {
			err := conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return // 连接断开，退出
			}
		}
	}()

	sendJSON := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		// 使用带超时的写入，防止阻塞
		select {
		case writeCh <- b:
		case <-time.After(100 * time.Millisecond):
			// 写入超时，丢弃
		}
	}

	// 1. 发送历史记录
	if len(pt.History) > 0 {
		sendJSON(map[string]any{
			"type": "output",
			"data": string(pt.History),
		})
	}

	// 2. 监听新的 SSH 输出
	// 创建一个退出信号通道
	done := make(chan struct{})

	go func() {
		defer close(writeCh) // 退出时关闭写入通道
		for {
			select {
			case data, ok := <-pt.OutputChan:
				if !ok {
					return // 通道关闭
				}
				sendJSON(map[string]any{
					"type": "output",
					"data": string(data),
				})
			case <-done:
				return // WebSocket 断开
			}
		}
	}()

	// 3. 从 WebSocket 读取并写入 SSH stdin
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// 每次收到消息都刷新读取超时
		err = conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			break
		}

		var in inboundMsg
		if err := json.Unmarshal(data, &in); err != nil {
			// 兼容：如果前端直接发原始输入（非JSON）
			if _, err := pt.Stdin.Write(data); err != nil {
				break
			}
			continue
		}

		switch in.Type {
		case "resize":
			if in.Cols > 0 && in.Rows > 0 {
				_ = ctrl.terminalService.ResizeTerminal(userID, isRoot, in.Cols, in.Rows)
			}
		case "input":
			if _, err := pt.Stdin.Write([]byte(in.Data)); err != nil {
				goto EndLoop
			}
		default:
			// 默认作为输入
			if _, err := pt.Stdin.Write([]byte(in.Data)); err != nil {
				goto EndLoop
			}
		}
	}

EndLoop:
	close(done)    // 通知输出协程退出
	writeWg.Wait() // 等待写入协程结束
}
