package ws

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册WebSocket路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ws/connect", ctrl.Connect)
}
