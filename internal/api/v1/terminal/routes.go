package terminal

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册终端路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	terminalGroup := r.Group("/terminal")
	{
		// WebSocket 终端透传：连接时通过 Query token= 或 Header Authorization 认证
		terminalGroup.GET("/ws", ctrl.WebSocketTerminal)
		// 兼容 API3.0.md 所述 GET /api/terminal/connect
		terminalGroup.GET("/connect", ctrl.WebSocketTerminal)
	}
}
