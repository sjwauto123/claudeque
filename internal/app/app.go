package app

import (
	"cloudque/pkg/websocket"
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloudque/internal/api"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/internal/service"
	"cloudque/pkg/config"
	"cloudque/pkg/database"
	"cloudque/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用结构体
type App struct {
	cfg       *config.Config
	mysqlDB   *gorm.DB
	redis     *redis.Client
	router    *api.Router
	server    *http.Server
	scheduler service.Scheduler
	pool    *websocket.ConnectionPool
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

	// 4. 初始化 WebSocket连接池
	a.pool = websocket.NewConnectionPool()

	// 5. 初始化依赖
	a.initDependencies()

	// 6. 初始化路由
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
		&entity.OperationLog{},
		&entity.BaseEntity{},
		&entity.Process{},
		&entity.User{},
		&entity.Job{},
		&entity.GpuCard{},
		&entity.Role{},
		&entity.Permission{},
	); err != nil {
		logger.Warn("数据库迁移警告", zap.Error(err))
	} else {
		logger.Info("数据库迁移完成")
	}

	// 初始化 Redis
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("Redis 初始化失败，将不影响核心功能", zap.Error(err))
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
	operationLogRepo := repository.NewOperationLogRepository(a.mysqlDB)
	processRepo := repository.NewProcessRepository(a.mysqlDB)
	userRepo := repository.NewUserRepository(a.mysqlDB)
	jobRepo := repository.NewJobRepository(a.mysqlDB, a.redis)
	gpuRepo := repository.NewGpuRepository(a.mysqlDB)
	gpuCache := repository.NewGpuCacheRepository(a.redis)
	queueRepo := repository.NewQueueRepository(a.redis)
	roleRepo := repository.NewRoleRepository(a.mysqlDB)
	redisRepo := repository.NewRedisRepository()

	// 创建 Service
	operationLogSvc := service.NewOperationLogService(operationLogRepo)
	queueSvc := service.NewQueueService(queueRepo, jobRepo)
	gpuSvc := service.NewGpuService(gpuRepo, gpuCache)
	jobSvc := service.NewJobService(jobRepo, queueSvc, gpuSvc, userRepo)
	userLogSvc := service.NewUserOperationLogService(operationLogRepo)
	infoService := service.NewSystemInfoService(a.pool, processRepo)
	userSvc := service.NewUserService(userRepo, redisRepo)
	authSvc := service.NewAuthService(userRepo, roleRepo, redisRepo, userSvc)

	// 创建调度器
	a.scheduler = service.NewScheduler(jobRepo, queueSvc, gpuSvc, processRepo)
	// 创建 Router
	a.router = api.NewRouter(userSvc, authSvc, jobSvc, queueSvc, operationLogSvc, jobRepo)
	a.router = api.NewRouter(userLogSvc, infoService, userSvc, authSvc)

	// 启动日志限制定时任务
	go a.startLogLimitTask(userLogSvc)
}

// startLogLimitTask 启动日志限制定时任务
func (a *App) startLogLimitTask(adminLogSvc service.UserOperationLogService) {
	// 日志保留数量限制
	const logLimit int64 = 10000

	// 立即执行一次
	if err := adminLogSvc.LimitLogs(logLimit); err != nil {
		logger.Errorf("日志限制失败: %v", err)
	}

	// 创建定时器，每天执行一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := adminLogSvc.LimitLogs(logLimit); err != nil {
			logger.Errorf("日志限制失败: %v", err)
		} else {
			logger.Info("操作日志限制执行成功，保留最近10000条日志")
		}
	}
}

// Shutdown 关闭应用
func (a *App) Shutdown() {
	// 关闭 ConnectionPool
	if a.pool != nil {
		a.pool.CloseAll()
	}
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
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
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
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	// 停止任务调度器
	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭失败", zap.Error(err))
	}

	// 关闭数据库连接
	_ = database.CloseMySQL()
	_ = database.CloseRedis()

	// 同步日志
	_ = logger.Sync()

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}
