package user

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册用户路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	userGroup := r.Group("/user")
	userGroup.Use(middleware.Auth())
	userGroup.Use(middleware.RequirePermission(ctrl.authService))
	userGroup.Use(middleware.UserOperationLogs(ctrl.useOperationLogService))
	{
		userGroup.GET("/profile", middleware.WithOperation("查询用户资料"), ctrl.GetProfile)
		userGroup.PUT("/password", middleware.WithOperation("修改密码"), ctrl.ChangePassword)
		userGroup.GET("/list", middleware.WithOperation("获取用户列表"), ctrl.ListUsers)
		userGroup.GET("/by-username", middleware.WithOperation("根据用户名查询"), ctrl.GetByUsername)
		userGroup.POST("/avatar", middleware.WithOperation("上传头像"), ctrl.UploadAvatar)
	}
}
