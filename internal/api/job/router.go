package job

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// JobsRoutes 任务路由
func (ctrl *Controller) JobsRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	{
		r.POST("", middleware.RequirePermission(ctrl.authService, ""), ctrl.SubmitJob)
		r.GET("", middleware.RequirePermission(ctrl.authService, ""), ctrl.GetJobsList)
		r.GET("/wait", middleware.RequirePermission(ctrl.authService, ""), ctrl.GetWaitJobsList)
		r.GET("/stats", middleware.RequirePermission(ctrl.authService, ""), ctrl.GetStats)
		r.DELETE("/:id", middleware.RequirePermission(ctrl.authService, ""), ctrl.CancelJob)
	}
}
