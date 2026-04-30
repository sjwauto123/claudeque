package home

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *HomeController) RegisterRoutes(r *gin.RouterGroup) {
	homeGroup := r.Group("/home")
	homeGroup.Use(middleware.Auth())
	homeGroup.Use(middleware.RequirePermission(ctrl.authService))
	homeGroup.Use(middleware.CaptureRawBody())
	homeGroup.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		homeGroup.GET("/overview", middleware.WithOperation("获取首页信息"), ctrl.Overview)
		homeGroup.GET("/ws", middleware.WithOperation("首页实时信息"), ctrl.WebSocketOverview)
	}
}
