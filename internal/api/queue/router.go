package queue

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// QueueRoutes 队列路由
func (ctrl *Controller) QueueRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	{
		r.GET("", ctrl.GetQueue)
		r.POST("", ctrl.ReorderQueue)
		r.DELETE("/:jobId", ctrl.RemoveJob)
	}
}
