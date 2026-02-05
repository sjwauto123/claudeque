package service

import (
	"context"
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

// GpuStatus GPU状态信息
type GpuStatus struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Status       int    `json:"status"`         // 0-空闲，1-忙碌
	CurrentJobID *uint  `json:"current_job_id"` // 当前执行的任务ID
}

// InitializeGpus 初始化GPU卡片（如果不存在）
func (s *GpuService) InitializeGpus(ctx context.Context) error {
	// 检查是否已经初始化
	var count int64
	if err := s.db.Model(&entity.GpuCard{}).Count(&count).Error; err != nil {
		return fmt.Errorf("检查GPU表失败: %w", err)
	}

	// 如果已经有数据，则跳过
	if count >= int64(totalGpuCount) {
		return nil
	}

	// 清空旧数据（如果有）
	if count > 0 {
		if err := s.db.Exec("DELETE FROM gpu_cards").Error; err != nil {
			return fmt.Errorf("清空GPU表失败: %w", err)
		}
	}

	// 创建4张显卡
	for i := 0; i < totalGpuCount; i++ {
		gpu := entity.GpuCard{
			Name:   fmt.Sprintf("gpu-%d", i),
			Status: entity.GpuStatusIdle,
		}
		if err := s.db.Create(&gpu).Error; err != nil {
			return fmt.Errorf("创建GPU-%d失败: %w", i, err)
		}
		// 初始化Redis缓存
		key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, gpu.ID)
		s.redis.HSet(ctx, key, "status", entity.GpuStatusIdle)
		s.redis.HSet(ctx, key, "job_id", "")
		s.redis.Expire(ctx, key, 24*time.Hour)
	}

	return nil
}

// GetIdleCards 获取空闲显卡列表
func (s *GpuService) GetIdleCards(ctx context.Context) ([]entity.GpuCard, error) {
	var cards []entity.GpuCard
	err := s.db.Where("status = ?", entity.GpuStatusIdle).Find(&cards).Error
	if err != nil {
		return nil, fmt.Errorf("查询空闲显卡失败: %w", err)
	}
	return cards, nil
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

// GetBusyCards 获取忙碌显卡列表
func (s *GpuService) GetBusyCards(ctx context.Context) ([]entity.GpuCard, error) {
	var cards []entity.GpuCard
	err := s.db.Where("status = ?", entity.GpuStatusBusy).Find(&cards).Error
	if err != nil {
		return nil, fmt.Errorf("查询忙碌显卡失败: %w", err)
	}
	return cards, nil
}

// GetAllStatus 获取所有显卡状态
func (s *GpuService) GetAllStatus(ctx context.Context) ([]GpuStatus, error) {
	var cards []entity.GpuCard
	if err := s.db.Find(&cards).Error; err != nil {
		return nil, fmt.Errorf("查询所有显卡失败: %w", err)
	}

	statusList := make([]GpuStatus, len(cards))
	for i, card := range cards {
		statusList[i] = GpuStatus{
			ID:           card.ID,
			Name:         card.Name,
			Status:       card.Status,
			CurrentJobID: card.CurrentJobID,
		}
	}
	return statusList, nil
}

// AcquireCards 占用指定数量的显卡
// 返回被占用的显卡ID列表
func (s *GpuService) AcquireCards(ctx context.Context, count int, jobID uint) ([]uint, error) {
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
	cardIDs := make([]uint, len(cards))
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
func (s *GpuService) ReleaseCards(ctx context.Context, cardIDs []uint) error {
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

// ReleaseCardsByJobID 根据任务ID释放相关显卡
func (s *GpuService) ReleaseCardsByJobID(ctx context.Context, jobID uint) error {
	// 查找该任务占用的显卡
	var cards []entity.GpuCard
	if err := s.db.Where("current_job_id = ?", jobID).Find(&cards).Error; err != nil {
		return fmt.Errorf("查找任务%d的显卡失败: %w", jobID, err)
	}

	if len(cards) == 0 {
		return nil // 没有关联显卡
	}

	// 释放显卡
	cardIDs := make([]uint, len(cards))
	for i, card := range cards {
		cardIDs[i] = card.ID
	}

	return s.ReleaseCards(ctx, cardIDs)
}

// GetTotalGpuCount 获取总显卡数量
func (s *GpuService) GetTotalGpuCount() int {
	return totalGpuCount
}
