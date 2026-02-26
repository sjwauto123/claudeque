package role

import (
	"cloudque/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (ctrl *RoleController) RegisterRoutes(r *gin.RouterGroup) {
	roleGroup := r.Group("/permissionManage/roles")
	roleGroup.Use(middleware.Auth())
	roleGroup.Use(middleware.RequirePermission(ctrl.authService))
	roleGroup.Use(middleware.UserOperationLogs(ctrl.userLogService))
	{
		roleGroup.GET("/page", middleware.WithOperation("分页获取角色列表"), ctrl.PageList)
		roleGroup.GET("/:id", middleware.WithOperation("获取角色"), ctrl.GetRoleByID)
		roleGroup.POST("/", middleware.WithOperation("创建角色"), ctrl.Create)
		roleGroup.PUT("/", middleware.WithOperation("更新角色"), ctrl.Update)
		roleGroup.DELETE("/:id", middleware.WithOperation("删除角色"), ctrl.Delete)
		roleGroup.DELETE("/batch", middleware.WithOperation("批量删除角色"), ctrl.BatchDelete)
		roleGroup.GET("/role_perm/:id", middleware.WithOperation("获取角色权限"), ctrl.GetRolePermissionByID)
		roleGroup.PUT("/role_perm/:role_id", middleware.WithOperation("更新角色权限"), ctrl.UpdateRolePermission)
	}

}
