package api

import (
	"cloudque/internal/api/admin"
	"cloudque/internal/api/auth"
	"cloudque/internal/api/user"
	"cloudque/internal/middleware"
	"cloudque/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	userCtrl    *user.Controller
	authCtrl    *auth.Controller
	adminCtrl   *admin.Controller
	authService service.AuthService
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
) *Router {
	return &Router{
		userCtrl:    user.NewController(userService),
		authCtrl:    auth.NewController(authService, userService),
		adminCtrl:   admin.NewController(userService, userService, authService),
		authService: authService,
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
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "CloudQue API is running",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由 (公开)
		r.authCtrl.RegisterRoutes(v1)

		// 需要认证的路由组
		authorized := v1.Group("/")
		authorized.Use(middleware.Auth())
		authorized.Use(middleware.RequirePermission(r.authService))
		{
			// 用户路由
			r.userCtrl.RegisterRoutes(authorized)

			// 管理员路由
			r.adminCtrl.RegisterRoutes(authorized)
		}
	}
}
