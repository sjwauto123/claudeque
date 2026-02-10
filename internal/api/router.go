package api

import (
	"cloudque/internal/api/job"
	"cloudque/internal/api/operationLogs"
	"cloudque/internal/api/queue"
	"cloudque/internal/api/system"
	"cloudque/internal/api/v1/admin"
	"cloudque/internal/api/v1/auth"
	"cloudque/internal/api/v1/user"
	"cloudque/internal/middleware"
	"cloudque/internal/repository"
	"cloudque/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	jobCtrl          *job.Controller
	queueCtrl        *queue.Controller
	operationLogCtrl *operationLogs.Controller
	systemInfoCtrl   *system.Controller
	userCtrl         *user.Controller
	authCtrl         *auth.Controller
	adminCtrl        *admin.Controller
}

// NewRouter 创建路由
func NewRouter(
	userLogService service.UserOperationLogService,
	infoService service.SystemInfoService,
	userService service.UserService,
	authService service.AuthService,
	jobService service.JobService,
	queueService service.QueueService,
	repository repository.JobRepository,
) *Router {
	return &Router{
		userCtrl:         user.NewController(userService),
		authCtrl:         auth.NewController(authService, userService),
		adminCtrl:        admin.NewController(userService, userService, authService),
		operationLogCtrl: operationLogs.NewController(userLogService, authService),
		systemInfoCtrl:   system.NewController(infoService, authService),
		jobCtrl:          job.NewController(jobService, userLogService, authService),
		queueCtrl:        queue.NewController(queueService, repository, userLogService, authService),
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
		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 管理员路由
		r.adminCtrl.RegisterRoutes(v1)
	}
	//API v2 路由组,用户操作日志输出
	v2 := engine.Group("/api/operationLogs")

	{
		r.operationLogCtrl.RegisterRoutes(v2)
	}
	//API v3 路由组，展示系统信息
	v3 := engine.Group("/api/system")
	{
		r.systemInfoCtrl.RegisterRoutes(v3)
	}
	// api v4 路由组，展示任务信息
	v4 := engine.Group("/api/job")
	{
		r.jobCtrl.JobsRoutes(v4)
	}
	// api v5 路由组，展示队列信息
	v5 := engine.Group("/api/queue")
	{
		r.queueCtrl.QueueRoutes(v5)
	}
}
