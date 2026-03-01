package terminal

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册终端路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	r := router.Group("/terminal")
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	r.Use(middleware.UserOperationLogs(ctrl.userLogService))

	// 终端 WebSocket 连接
	r.GET("/ws", middleware.WithOperation("连接终端"), ctrl.WebSocketTerminal)

}
