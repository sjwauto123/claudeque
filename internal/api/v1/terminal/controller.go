package terminal

import (
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"encoding/json"
	"io"
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

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	var sessionErr error
	var wg sync.WaitGroup

	// 创建终端会话
	// 默认大小 80x24，后续可通过 resize 消息调整
	ts, err := ctrl.terminalService.NewTerminalSession(userID, stdinReader, stdoutWriter, stderrWriter, isRoot, 80, 24)
	if err != nil {
		response.BizError(c, err)
		return
	}
	defer func(ts *server.TerminalSession) {
		err := ts.Close()
		if err != nil {
			return
		}
	}(ts)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func(stdoutWriter *io.PipeWriter) {
			err := stdoutWriter.Close()
			if err != nil {
				logger.Error("stdoutWriter关闭失败")
			}
		}(stdoutWriter)
		defer func(stderrWriter *io.PipeWriter) {
			err := stderrWriter.Close()
			if err != nil {
				logger.Error("stderrWriter关闭失败")
			}
		}(stderrWriter)

		// 等待会话结束
		sessionErr = ts.Session.Wait()
	}()

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
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	sendJSON := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		select {
		case writeCh <- b:
		default:
		}
	}

	// 将 SSH 输出写入 WebSocket（串行写）
	forwardOut := func(reader io.Reader) {
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				sendJSON(map[string]any{
					"type": "output",
					"data": string(buf[:n]),
				})
			}
			if err != nil {
				break
			}
		}
	}
	go forwardOut(stdoutReader)
	go forwardOut(stderrReader)

	// 从 WebSocket 读取并写入 SSH stdin
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// 每次收到消息都刷新读取超时
		err = conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return
		}

		var in inboundMsg
		if err := json.Unmarshal(data, &in); err != nil {
			// 兼容：如果前端直接发原始输入（非JSON）
			if _, err := stdinWriter.Write(data); err != nil {
				break
			}
			continue
		}

		switch in.Type {
		case "input":
			if _, err := stdinWriter.Write([]byte(in.Data)); err != nil {
				break
			}
		case "resize":
			if in.Cols > 0 && in.Rows > 0 {
				_ = ts.Resize(in.Cols, in.Rows)
			}
		case "ping":
			sendJSON(map[string]any{"type": "pong"})
		case "close":
			_ = stdinWriter.Close()
			goto end
		default:
			// unknown type: ignore
		}
	}

end:
	_ = stdinWriter.Close()
	wg.Wait()
	close(writeCh)
	writeWg.Wait()
	_ = sessionErr
}
