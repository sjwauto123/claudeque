package job

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// JobsRoutes 任务路由
func (ctrl *Controller) JobsRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	{
		r.POST("", ctrl.SubmitJob)
		r.GET("", ctrl.GetJobsList)
		r.GET("/wait", ctrl.GetWaitJobsList)
		r.GET("/stats", ctrl.GetStats)
		r.DELETE("/:id", ctrl.CancelJob)
		r.GET("/gpus", ctrl.GetGpus)
	}
}
