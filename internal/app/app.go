package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"cloudque/internal/api"
	"cloudque/internal/middleware"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/internal/service"
	"cloudque/pkg/config"
	"cloudque/pkg/database"
	"cloudque/pkg/logger"
	"cloudque/pkg/ssh"
	"cloudque/pkg/websocket"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DiskUsageSchedulerInterface 磁盘使用调度器接口
type DiskUsageSchedulerInterface interface {
	Start()
	Stop()
}

// App 应用结构体
type App struct {
	cfg                *config.Config
	mysqlDB            *gorm.DB
	redis              *redis.Client
	router             *api.Router
	server             *http.Server
	scheduler          service.Scheduler
	diskUsageScheduler DiskUsageSchedulerInterface
	sessionManager     *ssh.SessionManager
	wsPool             *websocket.ConnectionPool
	infoService        service.SystemInfoService
	homeService        service.HomeService
	terminalSvc        service.TerminalService
	logManager         *middleware.LogManager
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 1. 加载配置
	if err := a.initConfig(); err != nil {
		return err
	}

	// 2. 初始化日志
	if err := a.initLogger(); err != nil {
		return err
	}

	// 3. 初始化数据库
	if err := a.initDatabase(); err != nil {
		return err
	}

	// 4. 初始化依赖
	a.initDependencies()

	// 5. 初始化路由
	// 打印 SSH 状态
	if a.cfg.Server.Enabled {
		logger.Info("SSH 远程服务器配置已就绪",
			zap.String("host", a.cfg.Server.Host),
			zap.String("root_user", a.cfg.Server.RootUsername),
		)
	} else {
		logger.Warn("SSH 功能已在配置中禁用")
	}

	// 5. 初始化路由
	a.initRouter()

	// 7. 初始化服务器
	a.initServer()

	return nil
}

// initConfig 加载配置
func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	a.cfg = cfg
	return nil
}

// initLogger 初始化日志
func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}

	// 打印启动横幅
	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("欢迎使用 %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("版本: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("模式: %s", a.cfg.App.Mode))
	logger.Info("配置加载成功")
	logger.Info("=========================================")

	// Debug config
	logger.Info("Debug DB Config",
		zap.String("host", a.cfg.Database.MySQL.Host),
		zap.Int("port", a.cfg.Database.MySQL.Port),
		zap.String("user", a.cfg.Database.MySQL.Username),
	)

	return nil
}

// initDatabase 初始化数据库
func (a *App) initDatabase() error {
	// 初始化 MySQL
	mysqlDB, err := database.InitMySQL(&a.cfg.Database.MySQL)
	if err != nil {
		return fmt.Errorf("MySQL 初始化失败: %w", err)
	}
	a.mysqlDB = mysqlDB

	// 自动迁移数据库表
	logger.Info("开始数据库迁移...")
	if err := a.mysqlDB.AutoMigrate(
		&entity.UserOperationLog{},
		&entity.AdminOperationLog{},
		&entity.BaseEntity{},
		&entity.Process{},
		&entity.User{},
		&entity.Job{},
		&entity.GpuCard{},
		&entity.Role{},
		&entity.Permission{},
		&entity.Menu{},
	); err != nil {
		logger.Warn("数据库迁移警告", zap.Error(err))
	} else {
		logger.Info("数据库迁移完成")
	}

	// 初始化 Redis
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		panic("初始化redis失败")
	}
	a.redis = rs

	//初始化GPU卡片
	if a.redis != nil {
		gpuRepo := repository.NewGpuRepository(a.mysqlDB)
		gpuCache := repository.NewGpuCacheRepository(a.redis)
		gpuSvc := service.NewGpuService(gpuRepo, gpuCache)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := gpuSvc.InitializeGpus(ctx); err != nil {
			logger.Warn("GPU卡片初始化失败", zap.Error(err))
		} else {
			logger.Info("GPU卡片初始化完成")
		}
		cancel()
	}

	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
	// 创建 Repository
	userLogRepo := repository.NewUserOperationLogRepository(a.mysqlDB)
	adminLogRepo := repository.NewAdminOperationLogRepository(a.mysqlDB)
	processRepo := repository.NewProcessRepository(a.mysqlDB)
	userRepo := repository.NewUserRepository(a.mysqlDB)
	jobRepo := repository.NewJobRepository(a.mysqlDB, a.redis)
	gpuRepo := repository.NewGpuRepository(a.mysqlDB)
	gpuCache := repository.NewGpuCacheRepository(a.redis)
	queueRepo := repository.NewQueueRepository(a.redis)
	roleRepo := repository.NewRoleRepository(a.mysqlDB)
	sessionRepo := repository.NewSessionRepository(a.redis)
	redisRepo := repository.NewRedisRepository()
	menuRepo := repository.NewMenuRepository(a.mysqlDB)
	apiRepo := repository.NewAPIRepository(a.mysqlDB)
	procCacheRepo := repository.NewProcessCacheRepository(a.redis)
	systemInfoRepo := repository.NewSystemInfoRepository()

	// 创建 SSH 会话管理器
	var sessionManager *ssh.SessionManager
	var sshConfig *ssh.Config
	if a.cfg.Server.Enabled {
		logger.Info("初始化 SSH 会话管理器",
			zap.String("host", a.cfg.Server.Host),
			zap.String("base_path", a.cfg.Server.BasePath),
			zap.Duration("session_timeout", a.cfg.Server.SessionTimeout),
		)
		sshConfig = &ssh.Config{
			ServerHost:           a.cfg.Server.Host,
			RootUsername:         a.cfg.Server.RootUsername,
			RootPassword:         a.cfg.Server.RootPassword,         // 添加 Root 密码
			PrivateKeyPath:       a.cfg.Server.PrivateKeyPath,       // 从配置文件读取私钥路径
			PrivateKeyPassphrase: a.cfg.Server.PrivateKeyPassphrase, // 从配置文件读取私钥密码
			Timeout:              a.cfg.Server.Timeout,
			SessionTimeout:       a.cfg.Server.SessionTimeout,
		}
		sessionManager = ssh.NewSessionManager(sshConfig, logger.GetLogger())
		a.sessionManager = sessionManager

		// 启动会话超时清理定时器
		if a.cfg.Server.SessionTimeout > 0 {
			go sessionManager.StartCleanupTimer()
		}
	} else {
		logger.Info("SSH 服务器未启用，文件和终端功能将受限")
		sessionManager = nil
		sshConfig = nil
		a.sessionManager = nil
	}

	// 创建 WebSocket 连接池
	a.wsPool = websocket.NewConnectionPool()

	// 创建 Service
	userLogSvc := service.NewUserOperationLogService(userLogRepo)
	adminLogSvc := service.NewAdminOperationLogService(adminLogRepo)
	infoService := service.NewSystemInfoService(a.wsPool, systemInfoRepo, redisRepo, procCacheRepo)
	queueSvc := service.NewQueueService(queueRepo, jobRepo)
	gpuSvc := service.NewGpuService(gpuRepo, gpuCache)
	userSvc := service.NewUserService(userRepo, redisRepo, sshConfig)
	roleSvc := service.NewRoleService(roleRepo)
	apiSvc := service.NewAPIService(apiRepo)
	menuSvc := service.NewMenuService(menuRepo)
	authSvc := service.NewAuthService(userRepo, roleRepo, redisRepo, userSvc, sessionRepo, sessionManager, sshConfig)
	execSvc := service.NewExecService(sessionManager, authSvc)

	// 创建日志管理器
	a.logManager = middleware.NewLogManager(userLogSvc)

	if a.cfg.Server.Enabled {
		authSvc.SetSSHServerHost(a.cfg.Server.Host)
		authSvc.SetSSHTimeout(a.cfg.Server.Timeout)
	}
	fileSvc := service.NewFileService(sessionManager, authSvc, redisRepo, userRepo)

	// 初始化磁盘使用定时任务调度器
	diskUsageScheduler := service.NewDiskUsageScheduler(fileSvc)
	diskUsageScheduler.Start()
	a.diskUsageScheduler = diskUsageScheduler

	terminalSvc := service.NewTerminalService(sessionManager, authSvc, a.wsPool)
	a.terminalSvc = terminalSvc

	// 重新创建 jobSvc 以包含 execSvc
	jobSvc := service.NewJobService(jobRepo, queueSvc, gpuSvc, userRepo, execSvc, authSvc)
	homeSvc := service.NewHomeService(gpuRepo, jobRepo, systemInfoRepo, redisRepo, a.wsPool)

	// 创建调度器
	a.scheduler = service.NewScheduler(jobRepo, queueSvc, gpuSvc, processRepo, procCacheRepo, execSvc, authSvc)

	// 将调度器注入 jobSvc
	jobSvc.SetScheduler(a.scheduler)

	//创建系统信息管理器
	a.infoService = infoService
	a.homeService = homeSvc

	// 创建 Router
	a.router = api.NewRouter(
		userLogSvc,
		adminLogSvc,
		infoService,
		userSvc,
		authSvc,
		roleSvc,
		apiSvc,
		menuSvc,
		jobSvc,
		homeSvc,
		queueSvc,
		jobRepo,
		gpuSvc,
		fileSvc,
		terminalSvc,
		a.wsPool,
	)
}

// initRouter 初始化路由
func (a *App) initRouter() {
	// 设置 Gin 模式
	gin.SetMode(a.cfg.App.Mode)
}

// initServer 初始化 HTTP 服务器
func (a *App) initServer() {
	engine := gin.New()

	// 注册路由
	a.router.Setup(engine)

	// 创建 HTTP 服务器
	a.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:           engine,
		ReadHeaderTimeout: 60 * time.Second,  // 增加读取头超时，防范慢连接攻击
		IdleTimeout:       120 * time.Second, // 增加空闲连接超时，释放资源
		MaxHeaderBytes:    1 << 20,           // 1 MB
	}
}

// Run 运行应用
func (a *App) Run() {
	// 启动任务调度器
	if a.scheduler != nil {
		a.scheduler.Start()
	}

	// 启动 HTTP 服务器
	go func() {
		logger.Info("HTTP 服务器启动",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	a.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	// 关闭websocket连接池
	if a.wsPool != nil {
		a.wsPool.CloseAll()
	}

	// 关闭所有终端 PTY 和输出缓存
	if a.terminalSvc != nil {
		a.terminalSvc.CloseAllTerminals()
	}

	// 停止任务调度器
	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	// 停止磁盘使用定时任务调度器
	if a.diskUsageScheduler != nil {
		a.diskUsageScheduler.Stop()
	}

	// 停止系统信息服务
	if a.infoService != nil {
		a.infoService.Stop()
	}

	// 停止首页概览服务
	if a.homeService != nil {
		a.homeService.Stop()
	}

	// 关闭日志管理器，等待剩余日志写入
	if a.logManager != nil {
		a.logManager.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭失败", zap.Error(err))
	}

	// 关闭所有 SSH 会话
	if a.sessionManager != nil {
		a.sessionManager.CloseAll()
	}

	// 关闭数据库连接
	_ = database.CloseMySQL()
	_ = database.CloseRedis()

	// 同步日志
	_ = logger.Sync()

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}
