package terminal

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册终端路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {

	// 1. Root 终端 (需特定权限)
	rootGroup := r.Group("/root")
	rootGroup.Use(middleware.RequirePermission(ctrl.authService, "ssh:root"))
	{
		rootGroup.GET("/ws", ctrl.WebSocketTerminal)
	}

	userGroup := r.Group("/user")
	userGroup.Use(middleware.RequirePermission(ctrl.authService, "ssh:user"))
	{
		// 2. User 终端 (需普通权限)
		userGroup.GET("/ws", ctrl.WebSocketTerminal)
	}

	// WebSocket 终端透传：连接时通过 Query token= 或 Header Authorization 认证
	r.GET("/ws", ctrl.WebSocketTerminal)
	// 兼容 API3.0.md 所述 GET /api/terminal/connect
	r.GET("/connect", ctrl.WebSocketTerminal)

}
