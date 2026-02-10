package queue

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// QueueRoutes 队列路由
func (ctrl *Controller) QueueRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	{
		r.GET("", middleware.RequirePermission(ctrl.authService, ""), ctrl.GetQueue)
		r.POST("", middleware.RequirePermission(ctrl.authService, ""), ctrl.ReorderQueue)
		r.DELETE("/:jobId", middleware.RequirePermission(ctrl.authService, ""), ctrl.RemoveJob)
	}
}
