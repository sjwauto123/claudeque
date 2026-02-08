package repository

import (
	"cloudque/internal/model/entity"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gpuRepository struct {
	db *gorm.DB
}

// NewGpuRepository 创建GPU仓储实例
func NewGpuRepository(db *gorm.DB) GpuRepository {
	return &gpuRepository{db: db}
}

type gpuCacheRepository struct {
	redis *redis.Client
}

// NewGpuCacheRepository 创建GPU缓存仓储
func NewGpuCacheRepository(redis *redis.Client) GpuCacheRepository {
	return &gpuCacheRepository{redis: redis}
}

const (
	gpuStatusKeyPrefix = "gpu:status:"
)

func (r *gpuRepository) FindByName(ctx context.Context, name string) (*entity.GpuCard, error) {
	var card entity.GpuCard
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&card).Error; err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *gpuRepository) Create(ctx context.Context, gpu *entity.GpuCard) error {
	return r.db.WithContext(ctx).Create(gpu).Error
}

func (r *gpuRepository) GetIdleCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.GpuCard{}).Where("status = ?", entity.GpuStatusIdle).Count(&count).Error
	return count, err
}

func (r *gpuRepository) Acquire(ctx context.Context, count int, jobID int) ([]entity.GpuCard, error) {
	var cards []entity.GpuCard

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查找并锁定空闲显卡
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ?", entity.GpuStatusIdle).
			Limit(count).
			Find(&cards).Error; err != nil {
			return fmt.Errorf("查找空闲显卡失败: %w", err)
		}

		// 检查数量是否足够
		if len(cards) < count {
			return fmt.Errorf("空闲显卡不足，需要%d张，实际只有%d张", count, len(cards))
		}

		// 更新显卡状态
		for i := range cards {
			// 注意：这里必须使用 cards[i] 的指针或者 ID 来更新，确保更新的是正确对象
			if err := tx.Model(&cards[i]).
				Updates(map[string]interface{}{
					"status":         entity.GpuStatusBusy,
					"current_job_id": jobID,
				}).Error; err != nil {
				return fmt.Errorf("更新GPU-%d状态失败: %w", cards[i].ID, err)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return cards, nil
}

func (r *gpuRepository) Release(ctx context.Context, cardIDs []int) error {
	if len(cardIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.GpuCard{}).
		Where("id IN ?", cardIDs).
		Updates(map[string]interface{}{
			"status":         entity.GpuStatusIdle,
			"current_job_id": nil,
		}).Error
}

func (r *gpuCacheRepository) SetBusy(ctx context.Context, gpuID int, jobID int) error {
	key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, gpuID)
	pipe := r.redis.Pipeline()
	pipe.HSet(ctx, key, "status", entity.GpuStatusBusy)
	pipe.HSet(ctx, key, "job_id", jobID)
	// 每次更新都重置过期时间，防止长期运行的任务导致key过期
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *gpuCacheRepository) SetIdle(ctx context.Context, gpuID int) error {
	key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, gpuID)
	pipe := r.redis.Pipeline()
	pipe.HSet(ctx, key, "status", entity.GpuStatusIdle)
	pipe.HSet(ctx, key, "job_id", "")
	pipe.Expire(ctx, key, 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}
