package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloudque/internal/model/entity"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GpuService GPU管理服务
type GpuService struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewGpuService 创建GPU服务
func NewGpuService(db *gorm.DB, redis *redis.Client) *GpuService {
	return &GpuService{db: db, redis: redis}
}

const (
	totalGpuCount      = 4             // 总显卡数量
	gpuStatusKeyPrefix = "gpu:status:" // GPU状态Redis key前缀
)

// InitializeGpus 初始化GPU卡片
// 只补齐缺失的GPU，不修改现有状态（防止进程重启时丢失正在运行的任务状态）
func (s *GpuService) InitializeGpus(ctx context.Context) error {
	for i := 0; i < totalGpuCount; i++ {
		name := fmt.Sprintf("gpu-%d", i)
		var existing entity.GpuCard
		result := s.db.Where("name = ?", name).First(&existing)
		if result.Error == nil {
			continue
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("查询 GPU %s 失败: %w", name, result.Error)
		}
		gpu := entity.GpuCard{
			Name:   name,
			Status: entity.GpuStatusIdle,
		}
		if err := s.db.Create(&gpu).Error; err != nil {
			return fmt.Errorf("创建GPU-%d失败: %w", i, err)
		}
		key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, gpu.ID)
		s.redis.HSet(ctx, key, "status", entity.GpuStatusIdle)
		s.redis.HSet(ctx, key, "job_id", "")
		s.redis.Expire(ctx, key, 24*time.Hour)
	}

	return nil
}

// GetIdleCount 获取空闲显卡数量
func (s *GpuService) GetIdleCount(ctx context.Context) (int, error) {
	var count int64
	err := s.db.Model(&entity.GpuCard{}).Where("status = ?", entity.GpuStatusIdle).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("统计空闲显卡失败: %w", err)
	}
	return int(count), nil
}

// AcquireCards 占用指定数量的显卡
func (s *GpuService) AcquireCards(ctx context.Context, count int, jobID int) ([]int, error) {
	if count <= 0 || count > totalGpuCount {
		return nil, fmt.Errorf("显卡数量必须在1-%d之间", totalGpuCount)
	}
	// 使用事务确保原子性
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 查找空闲显卡
	var cards []entity.GpuCard
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("status = ?", entity.GpuStatusIdle).
		Limit(count).
		Find(&cards).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("查找空闲显卡失败: %w", err)
	}

	// 检查数量是否足够
	if len(cards) < count {
		tx.Rollback()
		return nil, fmt.Errorf("空闲显卡不足，需要%d张，实际只有%d张", count, len(cards))
	}

	// 更新显卡状态为忙碌
	cardIDs := make([]int, len(cards))
	for i, card := range cards {
		if err := tx.Model(&card).
			Updates(map[string]interface{}{
				"status":         entity.GpuStatusBusy,
				"current_job_id": jobID,
			}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("更新GPU-%d状态失败: %w", i, err)
		}
		cardIDs[i] = card.ID

		// 更新Redis缓存
		key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, card.ID)
		s.redis.HSet(ctx, key, "status", entity.GpuStatusBusy)
		s.redis.HSet(ctx, key, "job_id", jobID)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return cardIDs, nil
}

// ReleaseCards 释放指定显卡
func (s *GpuService) ReleaseCards(ctx context.Context, cardIDs []int) error {
	if len(cardIDs) == 0 {
		return nil
	}

	// 更新显卡状态为空闲
	if err := s.db.Model(&entity.GpuCard{}).
		Where("id IN ?", cardIDs).
		Updates(map[string]interface{}{
			"status":         entity.GpuStatusIdle,
			"current_job_id": nil,
		}).Error; err != nil {
		return fmt.Errorf("释放显卡失败: %w", err)
	}

	// 更新Redis缓存
	for _, cardID := range cardIDs {
		key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, cardID)
		s.redis.HSet(ctx, key, "status", entity.GpuStatusIdle)
		s.redis.HSet(ctx, key, "job_id", "")
	}

	return nil
}
