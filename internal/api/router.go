package api

import (
	"cloudque/internal/api/v1/admin"
	"cloudque/internal/api/v1/auth"
	"cloudque/internal/api/v1/user"
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
	apiCtrl  *permission.APIController
	menuCtrl *menu.MenuController
	userCtrl  *user.Controller
	authCtrl  *auth.Controller
	adminCtrl *admin.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	roleService service.RoleService,
	apiService service.APIService,
	menuService service.MenuService,
) *Router {
	return &Router{
		roleCtrl: role.NewRoleController(roleService),
		apiCtrl:  permission.NewAPIController(apiService),
		menuCtrl: menu.NewMenuController(menuService),
		userCtrl:  user.NewController(userService),
		authCtrl:  auth.NewController(authService, userService),
		adminCtrl: admin.NewController(userService, userService, authService),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())
	// 静态资源：上传文件
	engine.Static("/uploads", "./uploads")

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
		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 管理员路由
		r.adminCtrl.RegisterRoutes(v1)
	}
}
