package job

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// JobsRoutes 任务路由
func (ctrl *Controller) JobsRoutes(router *gin.RouterGroup) {
	r := router.Group("/job")
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	r.Use(middleware.CaptureRawBody())
	r.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		r.POST("", middleware.WithOperation("提交任务"), ctrl.SubmitJob)
		r.GET("", ctrl.GetJobsList)
		r.GET("/wait", ctrl.GetWaitJobsList)
		r.GET("/stats", ctrl.GetStats)
		r.DELETE("/:id", middleware.WithOperation("删除排队任务"), ctrl.CancelJob)
		r.GET("/gpus", ctrl.GetGpus)
		r.GET("/conda-envs", ctrl.GetCondaEnvs)
	}
}
