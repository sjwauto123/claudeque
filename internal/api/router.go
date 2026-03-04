package api

import (
	"cloudque/internal/api/v1/admin"
	"cloudque/internal/api/v1/auth"
	"cloudque/internal/api/v1/files"
	"cloudque/internal/api/v1/job"
	"cloudque/internal/api/v1/operationLogs"
	"cloudque/internal/api/v1/permissionManage/menu"
	"cloudque/internal/api/v1/permissionManage/permission"
	"cloudque/internal/api/v1/permissionManage/role"
	"cloudque/internal/api/v1/queue"
	"cloudque/internal/api/v1/system"
	"cloudque/internal/api/v1/terminal"
	"cloudque/internal/api/v1/user"
	"cloudque/internal/middleware"
	"cloudque/internal/repository"
	"cloudque/internal/service"
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
	jobCtrl          *job.Controller
	queueCtrl        *queue.Controller
	systemInfoCtrl   *system.Controller
	operationLogCtrl *operationLogs.Controller
	roleCtrl         *role.RoleController
	apiCtrl          *permission.APIController
	menuCtrl         *menu.MenuController
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
) *Router {
	return &Router{
		roleCtrl:         role.NewRoleController(roleService, authService, userOperationLogService),
		apiCtrl:          permission.NewAPIController(apiService, authService, userOperationLogService),
		menuCtrl:         menu.NewMenuController(menuService, authService, userOperationLogService),
		userCtrl:         user.NewController(userService, userOperationLogService, authService),
		authCtrl:         auth.NewController(authService, userService, userOperationLogService),
		adminCtrl:        admin.NewController(userService, userService, authService, userOperationLogService, adminOperationLogService),
		filesCtrl:        files.NewController(fileService, authService, userOperationLogService),
		terminalCtrl:     terminal.NewController(terminalService, authService, userOperationLogService, wsPool),
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
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "CloudQue API is running",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 管理员路由
		r.adminCtrl.RegisterRoutes(v1)

		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// API管理路由
		r.apiCtrl.RegisterRoutes(v1)

		// 菜单管理路由
		r.menuCtrl.RegisterRoutes(v1)

		// 角色路由
		r.roleCtrl.RegisterRoutes(v1)

		// 终端路由
		r.terminalCtrl.RegisterRoutes(v1)

		// 日志路由
		r.operationLogCtrl.RegisterRoutes(v1)

		// 系统路由
		r.systemInfoCtrl.RegisterRoutes(v1)

		// 任务路由
		r.jobCtrl.JobsRoutes(v1)

		// 队列路由
		r.queueCtrl.QueueRoutes(v1)

		// 文件路由
		r.filesCtrl.RegisterRoutes(v1)
	}

}

// Close 关闭所有路由连接
func (r *Router) Close() error {
	return nil
}
