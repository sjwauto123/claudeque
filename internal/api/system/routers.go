package system

import "github.com/gin-gonic/gin"

func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/cpuInfo", ctrl.HandleWebSocket)

}
