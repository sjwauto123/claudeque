package system

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	router := r.Group("/system")
	router.Use(middleware.Auth())
	router.Use(middleware.RequirePermission(ctrl.authService))

	// WebSocket 连接
	router.GET("", ctrl.HandleWebSocket)

	// GPU 任务管理
	router.POST("/:pid/terminate", ctrl.TerminateProcess)
	router.POST("/:pid/retain", ctrl.RetainProcess)
	router.DELETE("/:pid/retain", ctrl.CancelRetain)
	router.GET("/config", ctrl.GetConfig)
	router.PUT("/config", ctrl.UpdateConfig)
}
