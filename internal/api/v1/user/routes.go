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
	userGroup.Use(middleware.CaptureRawBody())
	userGroup.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		userGroup.GET("/profile", ctrl.GetProfile)
		userGroup.PUT("/profile", middleware.WithOperation("更新个人信息"), ctrl.UpdateProfile)
		userGroup.PUT("/password", middleware.WithOperation("修改密码"), ctrl.ChangePassword)
		userGroup.GET("/list", ctrl.ListUsers)
		userGroup.GET("/by-username", ctrl.GetByUsername)
		userGroup.POST("/avatar", middleware.WithOperation("上传头像"), ctrl.UploadAvatar)
		userGroup.GET("/menu-permission", ctrl.GetUserMenuPermission)
	}
}
