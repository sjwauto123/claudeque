package menu

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册菜单管理路由
func (ctrl *MenuController) RegisterRoutes(r *gin.RouterGroup) {
	menuGroup := r.Group("/permissionManage/menus")
	menuGroup.Use(middleware.Auth())
	menuGroup.Use(middleware.RequirePermission(ctrl.authService))
	menuGroup.Use(middleware.CaptureRawBody())
	menuGroup.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		menuGroup.GET("/page", middleware.WithOperation("分页查询菜单列表"), ctrl.PageList)
		menuGroup.GET("/:id", middleware.WithOperation("获取菜单信息"), ctrl.GetMenuByID)
		menuGroup.POST("", middleware.WithOperation("创建菜单"), ctrl.Create)
		menuGroup.PUT("", middleware.WithOperation("更新菜单"), ctrl.Update)
		menuGroup.DELETE("/:id", middleware.WithOperation("删除菜单"), ctrl.Delete)
		menuGroup.DELETE("/batch", middleware.WithOperation("批量删除菜单"), ctrl.BatchDelete)
	}
}
