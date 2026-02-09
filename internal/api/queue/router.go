package queue

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// QueueRoutes 队列路由
func (ctrl *Controller) QueueRoutes(r *gin.RouterGroup) {
	queueGroup := r.Group("/queue")
	queueGroup.Use(middleware.Auth())
	queueGroup.Use(middleware.RequirePermission(ctrl.authService, ""))
	{
		queueGroup.GET("", ctrl.GetQueue)
		queueGroup.POST("", ctrl.ReorderQueue)
		queueGroup.DELETE("/:jobId", ctrl.RemoveJob)
	}
}
