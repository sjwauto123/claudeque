package ws

import (
	"cloudque/internal/service"
	"cloudque/pkg/jwt"
	"cloudque/pkg/response"
	"cloudque/pkg/ssh"
	pkgws "cloudque/pkg/websocket"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Controller WebSocket控制器
type Controller struct {
	pool           *pkgws.ConnectionPool
	authService    service.AuthService
	sessionManager *ssh.SessionManager
}

// NewController 创建WebSocket控制器
func NewController(pool *pkgws.ConnectionPool, authService service.AuthService, sessionManager *ssh.SessionManager) *Controller {
	return &Controller{
		pool:           pool,
		authService:    authService,
		sessionManager: sessionManager,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Connect 建立WebSocket连接
// @Summary 建立WebSocket连接
// @Description 建立WebSocket连接，用于保持长连接
// @Tags WebSocket
// @Param token query string true "Token"
// @Router /api/v1/ws/connect [get]
func (ctrl *Controller) Connect(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) >= 8 && strings.HasPrefix(authHeader, "Bearer ") {
			token = authHeader[7:]
		}
	}
	if token == "" {
		response.Unauthorized(c, "请提供 token")
		return
	}

	claims, err := jwt.ParseToken(token)
	if err != nil {
		response.Unauthorized(c, "无效的 token")
		return
	}
	userID := claims.GetUserID()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	ctrl.pool.Add(userID, conn, &pkgws.SessionMetadata{UserID: userID, SessionType: "ws", CreatedAt: time.Now().Unix()})
	defer ctrl.pool.Remove(userID)

	// 保持连接
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
