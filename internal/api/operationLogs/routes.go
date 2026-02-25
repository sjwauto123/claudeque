package operationLogs

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	router.Use(middleware.Auth())
	router.Use(middleware.RequirePermission(ctrl.authService))
	// 系统关机重启操作日志路由
	adminGroup := router.Group("/adminLog")
	{
		adminGroup.GET("/list", ctrl.GetAdminLogs)
	}

	// 用户操作日志路由
	userGroup := router.Group("/userLog")
	{
		userGroup.GET("/list", ctrl.GetUserLogs)
	}
}
