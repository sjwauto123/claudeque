package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	wsPool "cloudque/pkg/websocket"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type homeService struct {
	jobRepo       repository.JobRepository
	systemInfoSvc SystemInfoService
	Pool          *wsPool.ConnectionPool
	Collector     *HomeOverviewCollector
}

type HomeOverviewCollector struct {
	Pool    *wsPool.ConnectionPool
	Service *homeService
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	Once    sync.Once
}

func NewHomeService(jobRepo repository.JobRepository, systemInfoSvc SystemInfoService, pool *wsPool.ConnectionPool) HomeService {
	svc := &homeService{
		jobRepo:       jobRepo,
		systemInfoSvc: systemInfoSvc,
		Pool:          pool,
	}
	svc.Collector = NewHomeOverviewCollector(pool, svc)
	svc.Collector.Start()
	return svc
}

func (s *homeService) Stop() {
	if s.Collector != nil {
		s.Collector.Stop()
	}
	logger.Info("首页概览服务已停止")
}

func (s *homeService) HandleHomeMessage(conn *websocket.Conn, userID int) {
	metadata := &wsPool.SessionMetadata{
		UserID:      userID,
		SessionType: "homeOverview",
		Role:        "admin",
		CreatedAt:   time.Now().Unix(),
	}

	s.Pool.Add(userID, conn, metadata, nil)
	logger.Infof("新的首页概览websocket连接已建立，用户ID: %d", userID)
}

func NewHomeOverviewCollector(pool *wsPool.ConnectionPool, service *homeService) *HomeOverviewCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &HomeOverviewCollector{
		Pool:    pool,
		Service: service,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (c *HomeOverviewCollector) Start() {
	c.Once.Do(func() {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-c.ctx.Done():
					logger.Info("首页概览采集器已停止")
					return
				case <-ticker.C:
					if !c.Pool.IsHavingHomeOverviewConnection() {
						continue
					}

					overview, err := c.Service.GetOverview(context.Background())
					if err != nil {
						logger.Errorf("首页概览采集失败: %v", err)
						continue
					}

					data, err := json.Marshal(overview)
					if err != nil {
						logger.Errorf("首页概览JSON转换失败: %v", err)
						continue
					}

					c.Pool.BroadcastToAdminsByType("homeOverview", data)
				}
			}
		}()
		logger.Info("首页概览采集器已启动")
	})
}

func (c *HomeOverviewCollector) Stop() {
	c.cancel()
	c.wg.Wait()
	logger.Info("首页概览采集器已完全停止")
}

func (s *homeService) GetOverview(ctx context.Context) (*response.HomeOverviewResponse, error) {
	// GPU 和进程信息都来自 system 模块已经初始化好的采集器，避免首页维护另一套 nvidia-smi 解析逻辑。
	collectCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	gpus := s.collectRealtimeGpus(collectCtx)

	queueSummary, err := s.jobRepo.GetHomeQueueSummary(ctx)
	if err != nil {
		return nil, err
	}

	serverProcesses := s.collectServerProcesses(collectCtx)

	return &response.HomeOverviewResponse{
		GpuSummary:      buildHomeGpuSummary(gpus, serverProcesses),
		Gpus:            gpus,
		ServerProcesses: serverProcesses,
		QueueSummary:    queueSummary,
	}, nil
}

func (s *homeService) systemCollector() (*ResourceCollector, bool) {
	// 不改动 systemInfo 模块的 service 代码，仅复用已创建的 systemInfoService 内部采集器。
	systemSvc, ok := s.systemInfoSvc.(*systemInfoService)
	if !ok || systemSvc == nil || systemSvc.collector == nil {
		logger.Warn("首页无法复用系统信息采集器")
		return nil, false
	}
	return systemSvc.collector, true
}

func (s *homeService) collectRealtimeGpus(ctx context.Context) []response.GPUInfoResponse {
	rc, ok := s.systemCollector()
	if !ok || rc.Repo == nil {
		return []response.GPUInfoResponse{}
	}

	gpus, err := rc.Repo.GetGPUInfo(ctx)
	if err != nil {
		logger.Warnf("首页GPU实时信息采集失败: %v", err)
		return []response.GPUInfoResponse{}
	}
	return gpus
}

func (s *homeService) collectServerProcesses(ctx context.Context) []response.ServerProcessInfo {
	rc, ok := s.systemCollector()
	if !ok {
		return []response.ServerProcessInfo{}
	}

	// 进程分类继续由 system 现有逻辑负责：ProcessCache 标识系统任务，Redis 标识保留任务。
	_, serverProcesses := rc.collectAndClassifyProcesses(ctx)
	fillRunningDurationSecs(serverProcesses)
	return serverProcesses
}

func buildHomeGpuSummary(gpus []response.GPUInfoResponse, processes []response.ServerProcessInfo) response.HomeGpuSummary {
	// 首页 GPU 总览按实际任务进程占用判断忙碌状态，不再使用显存占用判断，避免显存默认占用导致误判。
	summary := response.HomeGpuSummary{Total: len(gpus)}
	busyGpuNames := make(map[string]struct{})
	for _, proc := range processes {
		if proc.GPUname != "" {
			busyGpuNames[proc.GPUname] = struct{}{}
		}
	}

	summary.Busy = len(busyGpuNames)
	if summary.Busy > summary.Total {
		summary.Busy = summary.Total
	}
	summary.Idle = summary.Total - summary.Busy
	return summary
}

func fillRunningDurationSecs(processes []response.ServerProcessInfo) {
	for i := range processes {
		if processes[i].RunningDurationSecs > 0 {
			continue
		}
		if duration, err := time.ParseDuration(processes[i].Runtime); err == nil {
			processes[i].RunningDurationSecs = int(duration.Seconds())
		}
	}
}
