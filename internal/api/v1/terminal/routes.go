package terminal

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册终端路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	r.Use(middleware.UserOperationLogs(ctrl.userLogService))

	// 终端 WebSocket 连接 (不需要走通用权限校验，控制器内部会校验)
	r.GET("/terminal/ws", middleware.WithOperation("连接终端"), ctrl.WebSocketTerminal)

	// 其他终端 API 需要权限校验
	api := r.Group("/terminal")
	api.Use(middleware.RequirePermission(ctrl.authService))
	{
		// 这里可以放其他终端相关的 REST API
	}
}
