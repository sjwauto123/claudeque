package operationLogs

import "github.com/gin-gonic/gin"

func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/userLog", ctrl.GetUserLogs)
}
