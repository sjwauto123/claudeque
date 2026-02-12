package service

import (
	"cloudque/internal/model/dto/response"
	"context"
	"fmt"

	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"
	"cloudque/pkg/utils"

	"go.uber.org/zap"
)

// GpuService GPU管理服务
type gpuService struct {
	gpuRepo  repository.GpuRepository
	gpuCache repository.GpuCacheRepository
}

// NewGpuService 创建GPU服务
func NewGpuService(gpuRepo repository.GpuRepository, gpuCache repository.GpuCacheRepository) GpuService {
	return &gpuService{gpuRepo: gpuRepo, gpuCache: gpuCache}
}

// InitializeGpus 初始化GPU卡片
func (s *gpuService) InitializeGpus(ctx context.Context) error {
	return s.SyncGpuCards(ctx)
}

// SyncGpuCards 从系统同步GPU信息到数据库
func (s *gpuService) SyncGpuCards(ctx context.Context) error {
	logger.Info("开始同步系统GPU信息")

	gpus, err := utils.GetGpuInfo()
	if err != nil {
		return fmt.Errorf("获取系统GPU信息失败: %w", err)
	}

	if len(gpus) == 0 {
		logger.Warn("未检测到系统GPU")
		return nil
	}

	var cards []entity.GpuCard
	for _, info := range gpus {
		cards = append(cards, entity.GpuCard{
			UUID:    info.UUID,
			Name:    info.Name,
			Index:   info.Index,
			GpuType: info.Type,
			Memory:  info.Memory,
			Status:  entity.GpuStatusIdle,
		})
	}

	if err := s.gpuRepo.SyncCards(ctx, cards); err != nil {
		return fmt.Errorf("同步GPU信息到数据库失败: %w", err)
	}

	// 初始化缓存状态，确保数据库与缓存一致
	for _, card := range cards {
		dbCard, err := s.gpuRepo.FindByUUID(ctx, card.UUID)
		if err == nil {
			if dbCard.Status == entity.GpuStatusIdle {
				_ = s.gpuCache.SetIdle(ctx, dbCard.ID)
			} else if dbCard.Status == entity.GpuStatusBusy && dbCard.CurrentJobID != nil {
				_ = s.gpuCache.SetBusy(ctx, dbCard.ID, *dbCard.CurrentJobID)
			}
		}
	}

	logger.Info("系统GPU信息同步完成", zap.Int("count", len(gpus)))
	return nil
}

// GetGpuCardsByIDs 根据ID列表获取显卡信息
func (s *gpuService) GetGpuCardsByIDs(ctx context.Context, ids []int) ([]entity.GpuCard, error) {
	return s.gpuRepo.GetByIDs(ctx, ids)
}

// CheckAvailable 检查显卡是否可用
func (s *gpuService) CheckAvailable(ctx context.Context, cardIDs []int) (bool, error) {
	// 优先尝试从缓存检查
	available, err := s.gpuCache.CheckAvailable(ctx, cardIDs)
	if err == nil && !available {
		// 缓存明确显示不可用，直接返回
		return false, nil
	}
	// 如果缓存显示可用或者查询缓存出错，则回退到数据库进行准确检查
	return s.gpuRepo.CheckAvailable(ctx, cardIDs)
}

// AcquireCards 占用指定的显卡
func (s *gpuService) AcquireCards(ctx context.Context, cardIDs []int, jobID int) error {
	if len(cardIDs) == 0 {
		return fmt.Errorf("未指定显卡")
	}
	// 调用 Repository 执行数据库事务
	err := s.gpuRepo.Acquire(ctx, cardIDs, jobID)
	if err != nil {
		return err
	}

	for _, cardID := range cardIDs {
		// 更新Redis缓存
		if err := s.gpuCache.SetBusy(ctx, cardID, jobID); err != nil {
			logger.Warn("更新GPU缓存失败", zap.Int("card_id", cardID), zap.Error(err))
		}
	}

	return nil
}

// ReleaseCards 释放指定显卡
func (s *gpuService) ReleaseCards(ctx context.Context, cardIDs []int) error {
	if len(cardIDs) == 0 {
		return nil
	}
	// 更新数据库
	if err := s.gpuRepo.Release(ctx, cardIDs); err != nil {
		return fmt.Errorf("释放显卡失败: %w", err)
	}
	// 更新Redis缓存
	for _, cardID := range cardIDs {
		if err := s.gpuCache.SetIdle(ctx, cardID); err != nil {
			fmt.Printf("Warning: Failed to update cache for gpu-%d: %v\n", cardID, err)
		}
	}
	return nil
}

func (r *gpuService) GetGpus(ctx context.Context) (response.Gpus, error) {
	gpus, total, err := r.gpuRepo.GetGpus(ctx)
	if err != nil {
		return response.Gpus{}, err
	}
	for i := range gpus {
		gpus[i].Gb = gpus[i].Memory / 1024
	}
	gpuAll := response.Gpus{
		List:  gpus,
		Total: total,
	}
	return gpuAll, nil
}
