package repository

import (
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"context"
	"fmt"
	"strconv"
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

func (r *gpuRepository) FindByUUID(ctx context.Context, uuid string) (*entity.GpuCard, error) {
	var card entity.GpuCard
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&card).Error; err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *gpuRepository) GetByIDs(ctx context.Context, ids []int) ([]entity.GpuCard, error) {
	var cards []entity.GpuCard
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

func (r *gpuRepository) Acquire(ctx context.Context, cardIDs []int, jobID int) error {
	if len(cardIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		// 检查并锁定显卡
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&entity.GpuCard{}).
			Where("id IN ? AND status = ?", cardIDs, entity.GpuStatusIdle).
			Count(&count).Error; err != nil {
			return fmt.Errorf("检查显卡状态失败: %w", err)
		}

		if int(count) != len(cardIDs) {
			return fmt.Errorf("部分指定的显卡已被占用或不存在")
		}

		// 更新显卡状态
		if err := tx.Model(&entity.GpuCard{}).
			Where("id IN ?", cardIDs).
			Updates(map[string]interface{}{
				"status":         entity.GpuStatusBusy,
				"current_job_id": jobID,
			}).Error; err != nil {
			return fmt.Errorf("更新显卡状态失败: %w", err)
		}
		return nil
	})
}

func (r *gpuRepository) CheckAvailable(ctx context.Context, cardIDs []int) (bool, error) {
	if len(cardIDs) == 0 {
		return true, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.GpuCard{}).
		Where("id IN ? AND status = ?", cardIDs, entity.GpuStatusIdle).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return int(count) == len(cardIDs), nil
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

func (r *gpuRepository) SyncCards(ctx context.Context, cards []entity.GpuCard) error {
	if len(cards) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uuid"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "index", "gpu_type", "memory"}),
	}).Create(&cards).Error
}

func (r *gpuRepository) GetGpus(ctx context.Context) ([]response.GpuSpec, int, error) {
	var gpus []response.GpuSpec

	err := r.db.WithContext(ctx).Model(&entity.GpuCard{}).Select("id, name, gpu_type as type, memory").Order("id ASC").Scan(&gpus).Error
	if err != nil {
		return nil, 0, err
	}
	var total int64
	err = r.db.WithContext(ctx).Model(&entity.GpuCard{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	return gpus, int(total), nil
}

func (r *gpuRepository) GetAllCards(ctx context.Context) ([]entity.GpuCard, error) {
	var cards []entity.GpuCard
	if err := r.db.WithContext(ctx).Order("`index` ASC").Find(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
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

func (r *gpuCacheRepository) CheckAvailable(ctx context.Context, cardIDs []int) (bool, error) {
	if len(cardIDs) == 0 {
		return true, nil
	}

	pipe := r.redis.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(cardIDs))
	for i, id := range cardIDs {
		key := fmt.Sprintf("%s%d", gpuStatusKeyPrefix, id)
		cmds[i] = pipe.HGetAll(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return false, err
	}

	for _, cmd := range cmds {
		res, err := cmd.Result()
		if err != nil {
			// 如果缓存不存在，保守起见认为不可用，或者可以返回特定错误让上层查数据库
			return false, nil
		}
		// 如果缓存为空，说明没初始化
		if len(res) == 0 {
			return false, nil
		}
		// 检查状态
		statusStr, ok := res["status"]
		if !ok || statusStr != strconv.Itoa(entity.GpuStatusIdle) {
			return false, nil
		}
	}

	return true, nil
}
