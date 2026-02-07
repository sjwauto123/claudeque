package menu

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册菜单管理路由

func (ctrl *MenuController) RegisterRoutes(r *gin.RouterGroup) {
	menuGroup := r.Group("/permissionManage/menus")
	{
		menuGroup.GET("/page", ctrl.PageList)
		menuGroup.POST("/", ctrl.Create)
		menuGroup.PUT("/", ctrl.Update)
		menuGroup.GET("/:id", ctrl.GetMenuByID)
		menuGroup.DELETE("/:id", ctrl.Delete)
		menuGroup.DELETE("/batch", ctrl.BatchDelete)
	}
}
