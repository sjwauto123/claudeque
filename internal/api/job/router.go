package job

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// JobsRoutes 任务路由
func (ctrl *Controller) JobsRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	r.Use(middleware.UserOperationLogs(ctrl.userOperationLogService))
	{
		r.POST("", middleware.WithOperation("提交任务"), ctrl.SubmitJob)
		r.GET("", middleware.WithOperation("获取任务列表"), ctrl.GetJobsList)
		r.GET("/wait", middleware.WithOperation("获取排队任务列表"), ctrl.GetWaitJobsList)
		r.GET("/stats", middleware.WithOperation("任务统计"), ctrl.GetStats)
		r.DELETE("/:id", middleware.WithOperation("删除排队任务"), ctrl.CancelJob)
		r.GET("/gpus", middleware.WithOperation("获取所有GPU信息"), ctrl.GetGpus)
	}
}
