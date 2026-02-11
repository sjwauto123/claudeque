package admin

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	adminGroup := r.Group("/admin")
	//adminGroup.Use(middleware.Auth())
	adminGroup.Use(middleware.UserOperationLogs(ctrl.userOperationLogSer))
	//adminGroup.Use(middleware.RequirePermission(ctrl.authService, "user_manage")) // 需要用户管理权限

	{
		adminGroup.POST("/users", middleware.WithOperation("创建用户"), ctrl.CreateUser)
		adminGroup.DELETE("/users/:id", middleware.WithOperation("删除用户"), ctrl.DeleteUser)
		adminGroup.PUT("/users/:id", middleware.WithOperation("更新用户"), ctrl.UpdateUser)
		adminGroup.GET("/users/:id", middleware.WithOperation("获取用户信息"), ctrl.GetUser)
		adminGroup.GET("/users", middleware.WithOperation("获取用户列表"), ctrl.ListUsers)
		adminGroup.POST("/users", ctrl.CreateUser)
		adminGroup.DELETE("/users/:id", ctrl.DeleteUser)
		adminGroup.PUT("/users/:id", ctrl.UpdateUser)
		adminGroup.GET("/users/:id", ctrl.GetUser)
		adminGroup.GET("/users", ctrl.ListUsers)
		adminGroup.GET("/roles/simple", ctrl.ListRoleSimple)
		adminGroup.GET("/restart", ctrl.Restart)
		adminGroup.GET("/shutdown", ctrl.Shutdown)
	}
}
