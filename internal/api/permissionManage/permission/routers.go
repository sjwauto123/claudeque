package permission

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册API路由
// 参数: r *gin.RouterGroup - 路由组
func (ctrl *APIController) RegisterRoutes(r *gin.RouterGroup) {
	apiGroup := r.Group("/permissionManage/API")
	apiGroup.Use(middleware.RequirePermission(ctrl.authService))
	{
		apiGroup.GET("/page", ctrl.PageList)
		apiGroup.GET("/:id", ctrl.GetAPIByID)
		apiGroup.POST("/", ctrl.Create)
		apiGroup.PUT("/", ctrl.Update)
		apiGroup.DELETE("/:id", ctrl.Delete)
		apiGroup.DELETE("/batch", ctrl.BatchDelete)
	}
}
