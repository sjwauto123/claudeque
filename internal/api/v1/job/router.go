package job

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// JobsRoutes 任务路由
func (ctrl *Controller) JobsRoutes(r *gin.RouterGroup) {
	jobsGroup := r.Group("/job")
	jobsGroup.Use(middleware.Auth())
	{
		jobsGroup.POST("", ctrl.SubmitJob)
		jobsGroup.GET("", ctrl.GetJobsList)
		jobsGroup.GET("/wait", ctrl.GetWaitJobsList)
		jobsGroup.GET("/stats", ctrl.GetStats)
		jobsGroup.GET("/:jobId", ctrl.GetJobLog)
		jobsGroup.DELETE("/:id", ctrl.CancelJob)
	}
}
