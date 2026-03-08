package terminal

import (
	"cloudque/internal/middleware"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"cloudque/pkg/websocket"
	"github.com/gin-gonic/gin"
	gwebsocket "github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
)

// Controller 终端控制器
type Controller struct {
	terminalService service.TerminalService
	authService     service.AuthService
	userLogService  service.UserOperationLogService
	wsPool          *websocket.ConnectionPool
}

// NewController 创建终端控制器
func NewController(terminalService service.TerminalService, authService service.AuthService, userLogService service.UserOperationLogService, wsPool *websocket.ConnectionPool) *Controller {
	return &Controller{
		terminalService: terminalService,
		authService:     authService,
		userLogService:  userLogService,
		wsPool:          wsPool,
	}
}

var wsUpgrader = gwebsocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WebSocketTerminal 处理浏览器与 SSH 终端之间的双向通信。
// 它将 HTTP 连接升级为 WebSocket 并将其传递给终端服务。
func (ctrl *Controller) WebSocketTerminal(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	// 确定用户是否需要 root 终端。
	isRoot, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeTerminal)
	if err != nil {
		response.BizError(c, err)
		return
	}

	// 将 HTTP 连接升级为 WebSocket 连接。
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// 升级将写入响应，因此我们只需记录并返回。
		logger.Error("无法升级到WebSocket", zap.Error(err))
		return
	}

	// 服务层现在处理终端连接的整个生命周期。
	// 我们传递连接，服务将管理 SSH 会话和 I/O。
	// 初始大小为默认值，可以通过“resize”消息进行调整。
	if err := ctrl.terminalService.HandleTerminalConnection(userID, isRoot, 80, 24, conn); err != nil {
		logger.Error("处理终端连接失败", zap.Error(err))
		// 连接可能已经关闭或处于错误状态。
		// 我们不需要在这里写入响应，因为 websocket 握手已完成。
		_ = conn.Close() // 尝试清理。
	}

}
