package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	wsPool "cloudque/pkg/websocket"
	"context"
	"encoding/json"
	"strconv"
	"strings"
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
	gpus, gpuMap := s.collectRealtimeGpus(collectCtx)

	queueSummary, err := s.jobRepo.GetHomeQueueSummary(ctx)
	if err != nil {
		return nil, err
	}

	serverProcesses := s.collectServerProcesses(collectCtx, gpuMap)

	return &response.HomeOverviewResponse{
		GpuSummary:      buildHomeGpuSummary(gpus),
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

func (s *homeService) collectRealtimeGpus(ctx context.Context) ([]response.GPUInfoResponse, map[string]string) {
	rc, ok := s.systemCollector()
	if !ok || rc.Repo == nil {
		return []response.GPUInfoResponse{}, map[string]string{}
	}

	gpus, gpuMap, err := rc.Repo.GetGPUInfo(ctx)
	if err != nil {
		logger.Warnf("首页GPU实时信息采集失败: %v", err)
		return []response.GPUInfoResponse{}, map[string]string{}
	}
	return gpus, gpuMap
}

func (s *homeService) collectServerProcesses(ctx context.Context, gpuMap map[string]string) []response.ServerProcessInfo {
	rc, ok := s.systemCollector()
	if !ok {
		return []response.ServerProcessInfo{}
	}

	// 进程分类继续由 system 现有逻辑负责：ProcessCache 标识系统任务，Redis 标识保留任务。
	_, serverProcesses := rc.collectAndClassifyProcesses(ctx, gpuMap)
	fillRunningDurationSecs(serverProcesses)
	return serverProcesses
}

func buildHomeGpuSummary(gpus []response.GPUInfoResponse) response.HomeGpuSummary {
	// 首页 GPU 总览只基于 nvidia-smi 实时指标判断：利用率或显存占用大于 0 即认为忙碌。
	summary := response.HomeGpuSummary{Total: len(gpus)}
	for _, gpu := range gpus {
		if parseMetricNumber(gpu.Util) > 0 || parseMetricNumber(gpu.MemUsed) > 0 {
			summary.Busy++
		} else {
			summary.Idle++
		}
	}
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

func parseMetricNumber(value string) int {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "%")
	value = strings.TrimSuffix(value, "MB")
	value = strings.TrimSpace(value)
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
