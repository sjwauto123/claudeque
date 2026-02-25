package terminal

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册终端路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))

	// 终端 (通过 :mode 参数区分 root/user)
	terminalGroup := r.Group("/:mode")
	{
		terminalGroup.GET("/ws", ctrl.WebSocketTerminal)
	}

}
