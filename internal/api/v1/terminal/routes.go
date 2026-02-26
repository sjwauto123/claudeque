package terminal

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册终端路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	r.Use(middleware.UserOperationLogs(ctrl.userLogService))
	router := r.Group("/terminal")
	// 终端连接
	router.GET("/ws", middleware.WithOperation("连接终端"), ctrl.WebSocketTerminal)
}
