package terminal

import (
	"cloudque/pkg/logger"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"cloudque/internal/service"
	"cloudque/pkg/jwt"
	"cloudque/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Controller 终端控制器
type Controller struct {
	terminalService service.TerminalService
	authService     service.AuthService
}

// NewController 创建终端控制器
func NewController(terminalService service.TerminalService, authService service.AuthService) *Controller {
	return &Controller{
		terminalService: terminalService,
		authService:     authService,
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
	token := c.Query("token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) >= 8 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}
	if token == "" {
		response.Unauthorized(c, "请提供 token（Query token= 或 Authorization）")
		return
	}

	claims, err := jwt.ParseToken(token)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	userID := claims.GetUserID()
	if userID == 0 {
		response.Unauthorized(c, "无效用户")
		return
	}

	// 判断身份：根据 URL 参数决定是 root 终端还是普通用户终端
	mode := c.Param("mode")
	isRoot := (mode == "root")

	// 如果是 root 模式，需要检查用户是否拥有系统级权限
	if isRoot {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权访问 root 终端")
			return
		}
	}

	// 确保对应身份的 SSH 会话存在 (支持会话恢复)
	if err := ctrl.authService.EnsureSSHSessionByType(userID, isRoot); err != nil {
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
		sessionErr = ctrl.terminalService.RunInteractiveSession(userID, stdinReader, stdoutWriter, stderrWriter, isRoot)
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
		case "ping":
			sendJSON(map[string]any{"type": "pong"})
		case "resize":
			_ = ctrl.terminalService.ResizePTY(userID, in.Cols, in.Rows, isRoot)
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
