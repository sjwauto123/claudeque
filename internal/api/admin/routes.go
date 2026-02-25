package admin

import (
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	adminGroup := r.Group("/admin")

	{
		adminGroup.POST("/users", ctrl.CreateUser)
		adminGroup.DELETE("/users/:id", ctrl.DeleteUser)
		adminGroup.PUT("/users/:id", ctrl.UpdateUser)
		adminGroup.GET("/users/:id", ctrl.GetUser)
		adminGroup.GET("/users", ctrl.ListUsers)
		adminGroup.GET("/roles/simple", ctrl.ListRoleSimple)
		adminGroup.GET("/restart", ctrl.Restart)
		adminGroup.GET("/shutdown", ctrl.Shutdown)
	}
}
