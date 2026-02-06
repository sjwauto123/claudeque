package terminal

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

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
	logService      service.LogService
}

// NewController 创建终端控制器
func NewController(terminalService service.TerminalService, authService service.AuthService, logService service.LogService) *Controller {
	return &Controller{
		terminalService: terminalService,
		authService:     authService,
		logService:      logService,
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

	// 确保 SSH 会话存在 (支持会话恢复)
	if err := ctrl.authService.EnsureSSHSession(userID); err != nil {
		// 如果无法恢复会话（例如 Redis 中也没有凭证），则无法建立终端连接
		// WebSocket 握手阶段返回错误比较麻烦，通常直接关闭连接或返回 403
		response.BizError(c, err)
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 记录终端连接日志
	ctrl.logService.CreateLog(claims.GetUsername(), "TerminalConnect", "Terminal connected")
	defer func() {
		ctrl.logService.CreateLog(claims.GetUsername(), "TerminalDisconnect", "Terminal disconnected")
	}()

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	var sessionErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdoutWriter.Close()
		defer stderrWriter.Close()
		sessionErr = ctrl.terminalService.RunInteractiveSession(userID, stdinReader, stdoutWriter, stderrWriter)
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
			_ = ctrl.terminalService.ResizePTY(userID, in.Cols, in.Rows)
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
