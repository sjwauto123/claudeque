package repository

import (
	"cloudque/internal/model/entity"
	"context"
	"encoding/json"
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

type CachedProcessInfo struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

func (p *processCacheRepository) CreatePid(ctx context.Context, jobID int, PID int, jobName string) error {
	info := CachedProcessInfo{
		PID:  PID,
		Name: jobName,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return p.redis.HSet(ctx, KEY, strconv.Itoa(jobID), data).Err()
}

func (p *processCacheRepository) DelPid(ctx context.Context, jobID int) error {
	return p.redis.HDel(ctx, KEY, strconv.Itoa(jobID)).Err()
}

func (p *processCacheRepository) GetAllPid(ctx context.Context) ([]string, []int, error) {
	data, err := p.redis.HGetAll(ctx, KEY).Result()
	if err != nil {
		return nil, nil, err
	}

	names := make([]string, 0, len(data))
	pids := make([]int, 0, len(data))

	for k, v := range data {
		var info CachedProcessInfo
		if err := json.Unmarshal([]byte(v), &info); err == nil {
			names = append(names, info.Name)
			pids = append(pids, info.PID)
		} else {
			// 兼容旧数据: key是name, value是pid
			if pid, err := strconv.Atoi(v); err == nil {
				names = append(names, k)
				pids = append(pids, pid)
			}
		}
	}
	return names, pids, nil
}
