package system

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	router := r.Group("/system")
	router.Use(middleware.Auth())
	router.Use(middleware.RequirePermission(ctrl.authService))
	router.GET("", ctrl.HandleWebSocket)
}
