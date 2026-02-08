package service

import (
	"context"
	"errors"
	"fmt"

	"cloudque/internal/model/entity"
	"cloudque/internal/repository"

	"gorm.io/gorm"
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

const (
	totalGpuCount = 4 // 总显卡数量
)

// InitializeGpus 初始化GPU卡片
// 只补齐缺失的GPU，不修改现有状态（防止进程重启时丢失正在运行的任务状态）
func (s *gpuService) InitializeGpus(ctx context.Context) error {
	for i := 0; i < totalGpuCount; i++ {
		name := fmt.Sprintf("gpu-%d", i)
		_, err := s.gpuRepo.FindByName(ctx, name)
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("查询 GPU %s 失败: %w", name, err)
		}
		gpu := entity.GpuCard{
			Name:   name,
			Status: entity.GpuStatusIdle,
		}
		if err := s.gpuRepo.Create(ctx, &gpu); err != nil {
			return fmt.Errorf("创建GPU-%d失败: %w", i, err)
		}
		// 使用缓存层设置状态
		if err := s.gpuCache.SetIdle(ctx, gpu.ID); err != nil {
			return fmt.Errorf("初始化GPU-%d缓存失败: %w", i, err)
		}
	}

	return nil
}

// GetIdleCount 获取空闲显卡数量
func (s *gpuService) GetIdleCount(ctx context.Context) (int, error) {
	count, err := s.gpuRepo.GetIdleCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("统计空闲显卡失败: %w", err)
	}
	return int(count), nil
}

// AcquireCards 占用指定数量的显卡
func (s *gpuService) AcquireCards(ctx context.Context, count int, jobID int) ([]int, error) {
	if count <= 0 || count > totalGpuCount {
		return nil, fmt.Errorf("显卡数量必须在1-%d之间", totalGpuCount)
	}

	// 调用 Repository 执行数据库事务
	cards, err := s.gpuRepo.Acquire(ctx, count, jobID)
	if err != nil {
		return nil, err
	}

	cardIDs := make([]int, len(cards))
	for i, card := range cards {
		cardIDs[i] = card.ID

		// 更新Redis缓存
		if err := s.gpuCache.SetBusy(ctx, card.ID, jobID); err != nil {
			// 记录错误但不必回滚数据库，缓存可以容忍短暂不一致或通过过期修复
			fmt.Printf("Warning: Failed to update cache for gpu-%d: %v\n", card.ID, err)
		}
	}

	return cardIDs, nil
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
