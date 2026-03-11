package auth

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册认证路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("/auth")
	authGroup.Use(middleware.CaptureRawBody())
	authGroup.Use(middleware.GlobalLogManager.UserOperationLogs())
	{
		//注册
		authGroup.POST("/register", middleware.WithOperation("用户注册"), ctrl.Register)
		//登录
		authGroup.POST("/login", middleware.WithOperation("用户登录"), ctrl.Login)
		//登出
		authGroup.POST("/logout", middleware.Auth(), middleware.WithOperation("用户登出"), ctrl.Logout)
		//发送邮箱验证码
		authGroup.POST("/email/code", ctrl.SendEmailCode)
		//忘记密码
		authGroup.POST("/password/reset", ctrl.ResetPassword)
		//刷新token
		authGroup.POST("/refresh", ctrl.RefreshToken)
		//图形验证码
		authGroup.GET("/captcha", ctrl.GetCaptcha)
	}
}
