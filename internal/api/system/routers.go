package system

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	//router.Use(middleware.Auth())
	router.Use(middleware.RequirePermission(ctrl.authSvc, "getSystemInfo"))
	router.Use(middleware.UserOperationLogs(ctrl.userOperationLogService))
	router.GET("/cpuInfo", middleware.WithOperation("获取系统信息"), ctrl.HandleWebSocket)
}
