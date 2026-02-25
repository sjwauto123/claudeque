package admin

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.Auth())
	adminGroup.Use(middleware.RequirePermission(ctrl.authService, "user_manage")) // 需要用户管理权限
	adminGroup.Use(middleware.UserOperationLogs(ctrl.userOperationLogSer))
	{
		adminGroup.POST("/users", middleware.WithOperation("创建用户"), ctrl.CreateUser)
		adminGroup.DELETE("/users/:id", middleware.WithOperation("删除用户"), ctrl.DeleteUser)
		adminGroup.PUT("/users/:id", middleware.WithOperation("更新用户"), ctrl.UpdateUser)
		adminGroup.GET("/users/:id", middleware.WithOperation("获取用户信息"), ctrl.GetUser)
		adminGroup.GET("/users", middleware.WithOperation("获取用户列表"), ctrl.ListUsers)
		adminGroup.GET("/roles/simple", middleware.WithOperation("获取角色列表"), ctrl.ListRoleSimple)
		//开关机不用记录在用户操作日志中
		adminGroup.GET("/restart", ctrl.Restart)
		adminGroup.GET("/shutdown", ctrl.Shutdown)
	}

}
