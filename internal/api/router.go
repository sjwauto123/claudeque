package api

import (
	"cloudque/internal/api/admin"
	"cloudque/internal/api/auth"
	"cloudque/internal/api/files"
	"cloudque/internal/api/job"
	"cloudque/internal/api/operationLogs"
	"cloudque/internal/api/queue"
	"cloudque/internal/api/system"
	"cloudque/internal/api/terminal"
	"cloudque/internal/api/user"
	"cloudque/internal/api/ws"
	"cloudque/internal/middleware"
	"cloudque/internal/repository"
	"cloudque/internal/service"
	"cloudque/pkg/ssh"
	"cloudque/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// Router 路由

type Router struct {
	userCtrl         *user.Controller
	authCtrl         *auth.Controller
	adminCtrl        *admin.Controller
	filesCtrl        *files.Controller
	terminalCtrl     *terminal.Controller
	wsCtrl           *ws.Controller
	jobCtrl          *job.Controller
	queueCtrl        *queue.Controller
	systemInfoCtrl   *system.Controller
	operationLogCtrl *operationLogs.Controller
	roleCtrl *role.RoleController
	apiCtrl  *permission.APIController
	menuCtrl *menu.MenuController
}

// NewRouter 创建路由
func NewRouter(
	userOperationLogService service.UserOperationLogService,
	adminOperationLogService service.AdminOperationLogService,
	infoService service.SystemInfoService,
	userService service.UserService,
	authService service.AuthService,
	roleService service.RoleService,
	apiService service.APIService,
	menuService service.MenuService,
	jobService service.JobService,
	queueService service.QueueService,
	repository repository.JobRepository,
	gpuService service.GpuService,
	fileService service.FileService,
	terminalService service.TerminalService,
	wsPool *websocket.ConnectionPool,
	sessionManager *ssh.SessionManager,
) *Router {
	return &Router{
		roleCtrl: role.NewRoleController(roleService),
		apiCtrl:  permission.NewAPIController(apiService),
		menuCtrl: menu.NewMenuController(menuService),
		userCtrl:  user.NewController(userService),
		authCtrl:  auth.NewController(authService, userService),
		adminCtrl: admin.NewController(userService, userService, authService),
		userCtrl:         user.NewController(userService, userOperationLogService),
		authCtrl:         auth.NewController(authService, userService, userOperationLogService),
		adminCtrl:        admin.NewController(userService, userService, authService, userOperationLogService, adminOperationLogService),
		filesCtrl:        files.NewController(fileService, authService),
		terminalCtrl:     terminal.NewController(terminalService, authService),
		wsCtrl:           ws.NewController(wsPool, authService, sessionManager),
		jobCtrl:          job.NewController(jobService, authService, gpuService, userOperationLogService),
		queueCtrl:        queue.NewController(queueService, userOperationLogService, repository, authService),
		operationLogCtrl: operationLogs.NewController(adminOperationLogService, userOperationLogService, authService),
		systemInfoCtrl:   system.NewController(infoService, authService, userOperationLogService),
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
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 管理员路由
		r.adminCtrl.RegisterRoutes(v1)

		// API管理路由
		r.apiCtrl.RegisterRoutes(v1)

		// 菜单管理路由
		r.menuCtrl.RegisterRoutes(v1)
		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 角色路由
		r.roleCtrl.RegisterRoutes(v1)

	}

	//API v2 路由组 操作日志输出
	v2 := engine.Group("/api/operationLogs")
	{
		r.operationLogCtrl.RegisterRoutes(v2)
	}

	//API v3 路由组 展示系统信息
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

	// api v5 路由组，文件
	v6 := engine.Group("/api/files")
	{
		r.filesCtrl.RegisterRoutes(v6)
	}

	// api v5 路由组，展示队列信息
	v7 := engine.Group("/api/terminal")
	{
		// 终端路由
		r.terminalCtrl.RegisterRoutes(v7)

		// WebSocket 路由
		r.wsCtrl.RegisterRoutes(v7)
	}

}

// Close 关闭所有路由连接
func (r *Router) Close() error {
	return nil
}
