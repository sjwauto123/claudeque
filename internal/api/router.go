package api

import (
	"cloudque/internal/api/v1/auth"
	"cloudque/internal/api/v1/files"
	"cloudque/internal/api/v1/terminal"
	"cloudque/internal/api/v1/user"
	"cloudque/internal/api/v1/ws"
	"cloudque/internal/middleware"
	"cloudque/internal/service"
	"cloudque/pkg/ssh"
	"cloudque/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	userCtrl     *user.Controller
	authCtrl     *auth.Controller
	filesCtrl    *files.Controller
	terminalCtrl *terminal.Controller
	wsCtrl       *ws.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	fileService service.FileService,
	terminalService service.TerminalService,
	wsPool *websocket.ConnectionPool,
	sessionManager *ssh.SessionManager,
	logService service.LogService,
) *Router {
	return &Router{
		userCtrl:     user.NewController(userService),
		authCtrl:     auth.NewController(authService, userService, logService),
		filesCtrl:    files.NewController(fileService, logService),
		terminalCtrl: terminal.NewController(terminalService, authService, logService),
		wsCtrl:       ws.NewController(wsPool, authService, sessionManager),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "CloudQue API is running",
		})
	})

	// API 路由组（按 API3.0.md：/api）
	api := engine.Group("/api")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(api)

		// 用户路由
		r.userCtrl.RegisterRoutes(api)

		// 文件路由
		r.filesCtrl.RegisterRoutes(api)

		// 终端路由
		r.terminalCtrl.RegisterRoutes(api)

		// WebSocket 路由
		r.wsCtrl.RegisterRoutes(api)
	}
}

// Close 关闭所有路由连接
func (r *Router) Close() error {
	return nil
}
