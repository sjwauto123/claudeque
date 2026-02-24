package api

import (
	"cloudque/internal/api/job"
	"cloudque/internal/api/queue"
	"cloudque/internal/api/admin"
	"cloudque/internal/api/auth"
	"cloudque/internal/api/user"
	"cloudque/internal/middleware"
	"cloudque/internal/repository"
	"cloudque/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	jobCtrl   *job.Controller
	queueCtrl *queue.Controller
	userCtrl  *user.Controller
	authCtrl  *auth.Controller
	adminCtrl *admin.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	jobService service.JobService,
	queueService service.QueueService,
	repository repository.JobRepository,
	gpuService service.GpuService,
) *Router {
	return &Router{
		userCtrl:  user.NewController(userService),
		authCtrl:  auth.NewController(authService, userService),
		adminCtrl: admin.NewController(userService, userService, authService),
		jobCtrl:   job.NewController(jobService, authService, gpuService),
		queueCtrl: queue.NewController(queueService, repository, authService),
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

	engine.GET("/api/v1/buildtest", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "/api/v1/buildtestabc123456",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 管理员路由
		r.adminCtrl.RegisterRoutes(v1)
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
