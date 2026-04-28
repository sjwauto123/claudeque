package permission

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册API路由
func (ctrl *APIController) RegisterRoutes(r *gin.RouterGroup) {
	apiGroup := r.Group("/permissionManage/API")
	apiGroup.Use(middleware.Auth())
	apiGroup.Use(middleware.RequirePermission(ctrl.authService))
	apiGroup.Use(middleware.CaptureRawBody())
	apiGroup.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		apiGroup.GET("/page", ctrl.PageList)
		apiGroup.GET("/:id", ctrl.GetAPIByID)
		apiGroup.POST("", middleware.WithOperation("创建API"), ctrl.Create)
		apiGroup.PUT("", middleware.WithOperation("更新API"), ctrl.Update)
		apiGroup.DELETE("/:id", middleware.WithOperation("删除API"), ctrl.Delete)
		apiGroup.DELETE("/batch", middleware.WithOperation("批量删除API"), ctrl.BatchDelete)
	}
}
