package role

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *RoleController) RegisterRoutes(r *gin.RouterGroup) {
	roleGroup := r.Group("/permissionManage/roles")
	roleGroup.Use(middleware.RequirePermission(ctrl.authService))
	{
		roleGroup.GET("/page", ctrl.PageList)
		roleGroup.GET("/:id", ctrl.GetRoleByID)
		roleGroup.POST("/", ctrl.Create)
		roleGroup.PUT("/", ctrl.Update)
		roleGroup.DELETE("/:id", ctrl.Delete)
		roleGroup.DELETE("/batch", ctrl.BatchDelete)
		roleGroup.GET("/role_perm/:id", ctrl.GetRolePermissionByID)
		roleGroup.PUT("/role_perm/:role_id", ctrl.UpdateRolePermission)
	}

}
