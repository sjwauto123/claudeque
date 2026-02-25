package queue

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// QueueRoutes 队列路由
func (ctrl *Controller) QueueRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	r.Use(middleware.UserOperationLogs(ctrl.userOperationLogService))
	{
		r.GET("", middleware.WithOperation("获取排队队列"), ctrl.GetQueue)
		r.POST("", middleware.WithOperation("重新排序队列"), ctrl.ReorderQueue)
		r.DELETE("/:jobId", middleware.WithOperation("移除队列中的任务"), ctrl.RemoveJob)
	}
}
