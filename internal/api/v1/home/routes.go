package home

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *HomeController) RegisterRoutes(r *gin.RouterGroup) {
	homeGroup := r.Group("/home")
	homeGroup.Use(middleware.Auth())
	homeGroup.Use(middleware.RequirePermission(ctrl.authService))

	{
		homeGroup.GET("/overview", ctrl.Overview)
		homeGroup.GET("/ws", ctrl.WebSocketOverview)
	}
}
