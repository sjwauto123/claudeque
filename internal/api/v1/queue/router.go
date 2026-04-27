package queue

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// QueueRoutes 队列路由
func (ctrl *Controller) QueueRoutes(router *gin.RouterGroup) {
	r := router.Group("/queue")
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	r.Use(middleware.CaptureRawBody())
	r.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		r.GET("", ctrl.GetQueue)
		r.POST("", middleware.WithOperation("重新排序队列"), ctrl.ReorderQueue)
		r.DELETE("/:jobId", middleware.WithOperation("移除队列中的任务"), ctrl.RemoveJob)
	}
}
