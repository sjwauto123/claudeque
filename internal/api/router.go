package api

import (
	"cloudque/internal/api/permissionManage/permission"
	"cloudque/internal/api/permissionManage/role"
	"cloudque/internal/middleware"
	"cloudque/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	roleCtrl *role.RoleController
	apiCtrl  *permission.APIController
}

// NewRouter 创建路由
func NewRouter(
	roleService service.RoleService,
	apiService service.APIService,
) *Router {
	return &Router{
		roleCtrl: role.NewRoleController(roleService),
		apiCtrl:  permission.NewAPIController(apiService),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "CloudQue API is running",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api")
	{
		// 角色路由
		r.roleCtrl.RegisterRoutes(v1)

		// API管理路由
		r.apiCtrl.RegisterRoutes(v1)
	}
}
