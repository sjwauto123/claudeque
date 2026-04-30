package service

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	wsPool "cloudque/pkg/websocket"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type homeService struct {
	gpuRepo        repository.GpuRepository
	jobRepo        repository.JobRepository
	systemInfoRepo repository.SystemInfoRepository
	redisRepo      repository.RedisRepository
	Pool           *wsPool.ConnectionPool
	Collector      *HomeOverviewCollector
}

type HomeOverviewCollector struct {
	Pool    *wsPool.ConnectionPool
	Service *homeService
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	Once    sync.Once
}

type homeGpuMetric struct {
	Index       int
	UUID        string
	Temperature int
	Utilization int
	MemoryUsed  int
	MemoryTotal int
}

func NewHomeService(gpuRepo repository.GpuRepository, jobRepo repository.JobRepository, systemInfoRepo repository.SystemInfoRepository, redisRepo repository.RedisRepository, pool *wsPool.ConnectionPool) HomeService {
	svc := &homeService{
		gpuRepo:        gpuRepo,
		jobRepo:        jobRepo,
		systemInfoRepo: systemInfoRepo,
		redisRepo:      redisRepo,
		Pool:           pool,
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
	cards, err := s.gpuRepo.GetAllCards(ctx)
	if err != nil {
		return nil, err
	}

	metrics, err := collectHomeGpuMetrics()
	if err != nil {
		logger.Warnf("首页GPU实时指标采集失败: %v", err)
		metrics = map[int]homeGpuMetric{}
	}

	gpus := make([]response.HomeGPUOverview, 0, len(cards))
	summary := response.HomeGpuSummary{Total: len(cards)}
	for _, card := range cards {
		if card.Status == entity.GpuStatusBusy {
			summary.Busy++
		} else {
			summary.Idle++
		}

		gpu := response.HomeGPUOverview{
			ID:           card.ID,
			Index:        card.Index,
			Name:         card.Name,
			Type:         card.GpuType,
			Status:       card.Status,
			CurrentJobID: card.CurrentJobID,
			MemoryTotal:  card.Memory,
		}
		if metric, ok := metrics[card.Index]; ok {
			gpu.Temperature = metric.Temperature
			gpu.Utilization = metric.Utilization
			gpu.MemoryUsed = metric.MemoryUsed
			gpu.MemoryTotal = metric.MemoryTotal
		}
		gpus = append(gpus, gpu)
	}

	runningJobs, err := s.jobRepo.GetHomeRunningJobs(ctx)
	if err != nil {
		return nil, err
	}
	attachGpuNames(runningJobs, cards)

	queueSummary, err := s.jobRepo.GetHomeQueueSummary(ctx)
	if err != nil {
		return nil, err
	}

	serverProcesses := s.collectServerProcesses(ctx)

	return &response.HomeOverviewResponse{
		GpuSummary:      summary,
		Gpus:            gpus,
		ServerProcesses: serverProcesses,
		QueueSummary:    queueSummary,
	}, nil
}

func (s *homeService) collectServerProcesses(ctx context.Context) []response.ServerProcessInfo {
	if s.systemInfoRepo == nil || s.redisRepo == nil {
		return []response.ServerProcessInfo{}
	}

	_, gpuMap, err := s.systemInfoRepo.GetGPUInfo(ctx)
	if err != nil {
		logger.Warnf("首页GPU进程映射采集失败: %v", err)
		return []response.ServerProcessInfo{}
	}

	rc := &ResourceCollector{}
	_, serverProcesses := rc.collectAndClassifyProcesses(ctx, gpuMap)
	return serverProcesses
}

func collectHomeGpuMetrics() (map[int]homeGpuMetric, error) {
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,uuid,temperature.gpu,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	metrics := make(map[int]homeGpuMetric)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		temp, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
		util, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
		memUsed, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
		memTotal, _ := strconv.Atoi(strings.TrimSpace(parts[5]))

		metrics[index] = homeGpuMetric{
			Index:       index,
			UUID:        strings.TrimSpace(parts[1]),
			Temperature: temp,
			Utilization: util,
			MemoryUsed:  memUsed,
			MemoryTotal: memTotal,
		}
	}

	return metrics, nil
}

func attachGpuNames(jobs []response.HomeRunningJob, cards []entity.GpuCard) {
	cardNames := make(map[string]string, len(cards))
	for _, card := range cards {
		cardNames[strconv.Itoa(card.ID)] = card.Name
	}

	for i := range jobs {
		if jobs[i].GpuIDs == "" {
			continue
		}
		ids := strings.Split(jobs[i].GpuIDs, ",")
		names := make([]string, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if name, ok := cardNames[id]; ok {
				names = append(names, name)
			}
		}
		jobs[i].GpuNames = strings.Join(names, ",")
	}
}
