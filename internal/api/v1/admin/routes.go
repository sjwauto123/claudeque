package admin

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.Auth())                           // 需要认证
	adminGroup.Use(middleware.RequirePermission("user_manage")) // 需要用户管理权限

	{
		adminGroup.POST("/users", ctrl.CreateUser)
		adminGroup.DELETE("/users/:id", ctrl.DeleteUser)
		adminGroup.PUT("/users/:id", ctrl.UpdateUser)
		adminGroup.GET("/users/:id", ctrl.GetUser)
		adminGroup.GET("/users", ctrl.ListUsers)
	}
}
