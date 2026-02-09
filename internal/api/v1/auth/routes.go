package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册认证路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("/auth")
	{
		//注册
		authGroup.POST("/register", ctrl.Register)
		//登录
		authGroup.POST("/login", ctrl.Login)
		//刷新token
		authGroup.POST("/refresh", ctrl.RefreshToken)
		//发送邮箱验证码
		authGroup.POST("/email/code", ctrl.SendEmailCode)
		//忘记密码
		authGroup.POST("/password/reset", ctrl.ResetPassword)
		//图形验证码
		authGroup.GET("/captcha", ctrl.GetCaptcha)
	}
}
