package api

import (
	"cloudque/internal/api/system"
	"cloudque/internal/api/userLog"
	"cloudque/internal/service"
	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	userLogCtrl    *userLog.Controller
	systemInfoCtrl *system.Controller
}

// NewRouter 创建路由
func NewRouter(
	userLogService service.UserLogService,
	infoService service.SystemInfoService,
	// 新增
) *Router {
	return &Router{
		userLogCtrl:    userLog.NewController(userLogService),
		systemInfoCtrl: system.NewController(infoService),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	//engine.Use(middleware.Recovery())
	//engine.Use(middleware.Logger())
	//engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "CloudQue API is running",
		})
	})

	//API v2 路由组,用户操作日志输出
	v2 := engine.Group("/api/userLog")
	{
		r.userLogCtrl.RegisterRoutes(v2)
	}

	//API v3 路由组，展示系统信息
	v3 := engine.Group("/api/system")
	{
		r.systemInfoCtrl.RegisterRoutes(v3)
	}

}
