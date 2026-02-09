package operationLogs

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	router.Use(middleware.Auth())
	router.Use(middleware.RequirePermission(ctrl.authService, "getLogsInfo"))
	router.GET("/userLog", ctrl.GetUserLogs)
}
