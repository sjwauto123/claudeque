package repository

import (
	"cloudque/internal/model/entity"
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

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

func (p *processCacheRepository) CreatePid(ctx context.Context, PID int, jobName string) error {
	field := strconv.Itoa(PID)
	return p.redis.HSet(ctx, KEY, field, jobName).Err()
}

func (p *processCacheRepository) DelPid(ctx context.Context, PID int) error {
	field := strconv.Itoa(PID)
	return p.redis.HDel(ctx, KEY, field).Err()
}

func (p *processCacheRepository) GetAllPid(ctx context.Context) ([]string, []int, error) {
	// 使用 HGetAll 保证键值配对，避免 HVals/HKeys 顺序不一致
	all, err := p.redis.HGetAll(ctx, KEY).Result()
	if err != nil {
		return nil, nil, err
	}
	jobNames := make([]string, 0, len(all))
	pids := make([]int, 0, len(all))
	for k, v := range all {
		pid, err := strconv.Atoi(k)
		if err != nil {
			return nil, nil, fmt.Errorf("PID不合规：%w", err)
		}
		jobNames = append(jobNames, v)
		pids = append(pids, pid)
	}
	return jobNames, pids, nil
}
