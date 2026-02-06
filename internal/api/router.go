package api

import (
	"cloudque/internal/api/permissionManage/menu"
	"cloudque/internal/api/permissionManage/permission"
	"cloudque/internal/api/permissionManage/role"
	"cloudque/internal/middleware"
	"cloudque/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	roleCtrl *role.RoleController
	apiCtrl  *permission.APIController // 新增API控制器
	menuCtrl *menu.MenuController      // 新增菜单控制器
}

// NewRouter 创建路由
func NewRouter(
	roleService service.RoleService,
	apiService service.APIService, // 新增API服务
	menuService service.MenuService, // 新增菜单服务
) *Router {
	return &Router{
		roleCtrl: role.NewRoleController(roleService),
		apiCtrl:  permission.NewAPIController(apiService), // 新增API控制器初始化
		menuCtrl: menu.NewMenuController(menuService),     // 新增菜单控制器初始化
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

		// 菜单管理路由
		r.menuCtrl.RegisterRoutes(v1)
	}
}
