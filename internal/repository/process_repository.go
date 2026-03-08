package repository

import (
	"cloudque/internal/model/entity"
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"

	"gorm.io/gorm"
)

const KEY = "job:pid"

type processRepository struct {
	db *gorm.DB
}

func NewProcessRepository(db *gorm.DB) ProcessRepository {
	return &processRepository{db: db}
}

type processCacheRepository struct {
	redis *redis.Client
}

// NewProcessCacheRepository 创建GPU缓存仓储
func NewProcessCacheRepository(redis *redis.Client) ProcessCacheRepository {
	return &processCacheRepository{redis: redis}
}

// Create 创建进程信息
func (r *processRepository) Create(p *entity.Process) error {
	return r.db.Create(p).Error
}

// Update 更新进程信息 (通常用于记录结束时间)
func (r *processRepository) Update(p *entity.Process) error {
	return r.db.Save(p).Error
}

// FindActiveByJobID 根据任务ID获取活跃进程记录
func (r *processRepository) FindActiveByJobID(jobID int) ([]entity.Process, error) {
	var processes []entity.Process
	err := r.db.Where("job_id = ? AND ended_at IS NULL", jobID).Find(&processes).Error
	return processes, err
}

// FindAll 获取全部正在运行的进程
func (r *processRepository) FindAll() ([]entity.Process, error) {
	var processes []entity.Process
	err := r.db.Where("ended_at IS NULL").Find(&processes).Error
	return processes, err
}

func (p *processCacheRepository) CreatePid(ctx context.Context, PID int, jobID int) error {
	return p.redis.HSet(ctx, KEY, strconv.Itoa(jobID), PID).Err()
}

func (p *processCacheRepository) DelPid(ctx context.Context, jobID int) error {
	return p.redis.HDel(ctx, KEY, strconv.Itoa(jobID)).Err()
}

func (p *processCacheRepository) GetAllPid(ctx context.Context) ([]string, []int, error) {
	data, err := p.redis.HVals(ctx, KEY).Result()
	if err != nil {
		return nil, nil, err
	}
	keys, err := p.redis.HKeys(ctx, KEY).Result()
	if err != nil {
		return nil, nil, err
	}
	result := make([]int, len(data))
	for i, v := range data {
		result[i], err = strconv.Atoi(v)
		if err != nil {
			return nil, nil, fmt.Errorf("PID不合规：%w", err)
		}
	}
	return keys, result, nil
}
