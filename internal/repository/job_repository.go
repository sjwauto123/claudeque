package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type jobRepository struct {
	db *gorm.DB
	rs *redis.Client
}

// NewJobRepository 创建任务仓储
func NewJobRepository(db *gorm.DB, rs *redis.Client) JobRepository {
	return &jobRepository{db: db, rs: rs}
}

// GetJobList 获取任务列表
func (r *jobRepository) GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error) {
	return r.getJobListWithFilters(req, startTime, endTime, userID, req.Status)
}

// GetWaitJobList 获取正在排队的任务列表
func (r *jobRepository) GetWaitJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error) {
	statusFilter := []int{entity.JobStatusQueued, entity.JobStatusWaitingGpu}
	return r.getJobListWithFilters(req, startTime, endTime, userID, statusFilter)
}

// getJobListWithFilters 内部公共方法：根据过滤条件获取任务列表
func (r *jobRepository) getJobListWithFilters(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int, statusFilter interface{}) ([]response.JobResponse, int64, int, int, error) {
	list := make([]response.JobResponse, 0)
	var total int64

	baseDB := r.db.Table("jobs j").
		Where("j.user_id = ?", userID)

	// 状态过滤
	if statusFilter != nil {
		switch v := statusFilter.(type) {
		case int:
			if v > 0 {
				baseDB = baseDB.Where("j.status = ?", v)
			}
		case []int:
			if len(v) > 0 {
				baseDB = baseDB.Where("j.status IN ?", v)
			}
		}
	}

	// 条件
	if req.Id > 0 {
		baseDB = baseDB.Where("j.id = ?", req.Id)
	}
	if req.Name != "" {
		baseDB = baseDB.Where("j.name LIKE ?", "%"+req.Name+"%")
	}
	if !startTime.IsZero() {
		baseDB = baseDB.Where("j.created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		baseDB = baseDB.Where("j.created_at <= ?", endTime)
	}

	if err := baseDB.Count(&total).Error; err != nil {
		return []response.JobResponse{}, 0, req.Page, req.PageSize, err
	}

	offset := (req.Page - 1) * req.PageSize

	err := baseDB.
		Select(`j.id,j.name,j.description,j.status,j.created_at,j.gpu_ids AS card,0 AS count`).
		Order("j.created_at DESC").Limit(req.PageSize).Offset(offset).Scan(&list).Error

	if err != nil {
		return []response.JobResponse{}, 0, req.Page, req.PageSize, err
	}

	// 批量查询显卡信息
	gpuIDMap := make(map[string]string)
	var allGpuIDs []string

	// 收集所有需要查询的显卡ID
	for _, job := range list {
		if job.Card != "" {
			ids := strings.Split(job.Card, ",")
			allGpuIDs = append(allGpuIDs, ids...)
		}
	}

	// 如果有显卡ID，进行查询
	if len(allGpuIDs) > 0 {
		var gpuCards []entity.GpuCard
		if err := r.db.Table("gpu_cards").Where("id IN ?", allGpuIDs).Find(&gpuCards).Error; err == nil {
			for _, card := range gpuCards {
				gpuIDMap[strconv.Itoa(card.ID)] = card.Name
			}
		}
	}

	// 替换ID为名称
	for i := range list {
		if list[i].Card != "" {
			ids := strings.Split(list[i].Card, ",")
			var names []string
			for _, id := range ids {
				if name, ok := gpuIDMap[id]; ok {
					names = append(names, name)
				}
			}
			list[i].Card = strings.Join(names, ",")
		}
	}

	return list, total, req.Page, req.PageSize, nil
}

// Create 创建任务
func (r *jobRepository) Create(job *entity.Job) error {
	if err := r.db.Create(job).Error; err != nil {
		return err
	}
	return nil
}

// GetByID 根据ID获取任务
func (r *jobRepository) GetByID(id int) (*entity.Job, error) {
	var job entity.Job
	if err := r.db.First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("任务id为：%d 的任务不存在", id)
		}
		return nil, err
	}
	return &job, nil
}

// UpdateStatus 更新任务状态
func (r *jobRepository) UpdateStatus(id int, status int) error {
	now := time.Now()
	updates := map[string]interface{}{"status": status}

	// 根据状态设置相应的时间字段
	switch status {
	case entity.JobStatusRunning:
		updates["started_at"] = &now
	case entity.JobStatusCompleted, entity.JobStatusFailed, entity.JobStatusCancelled:
		updates["finished_at"] = &now
	}

	return r.db.Model(&entity.Job{}).Where("id = ?", id).Updates(updates).Error
}

// GetQueueJobsByIDs 通过任务id获取任务信息
func (r *jobRepository) GetQueueJobsByIDs(jobIDs []int) (map[int]response.QueueJobDBRow, error) {
	if len(jobIDs) == 0 {
		return map[int]response.QueueJobDBRow{}, nil
	}

	var rows []response.QueueJobDBRow

	err := r.db.Table("jobs j").
		Select(`j.id AS job_id,j.name AS job_name,j.description,j.status,j.created_at AS submitted_at,u.username AS user_name`).
		Joins("LEFT JOIN admin_users u ON u.id = j.user_id").Where("j.id IN ?", jobIDs).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make(map[int]response.QueueJobDBRow, len(rows))
	for _, r := range rows {
		result[r.JobID] = r
	}

	return result, nil
}

// GetQueueJobListFiltered 按队列顺序、条件筛选、分页获取排队任务
func (r *jobRepository) GetQueueJobListFiltered(orderedJobIDs []int, req request.QueueListRequest, startTime, endTime time.Time) ([]response.QueueJobDBRow, int, error) {
	if len(orderedJobIDs) == 0 {
		return []response.QueueJobDBRow{}, 0, nil
	}

	baseDB := r.buildBaseQueueJobQuery(req, startTime, endTime).
		Where("j.id IN ?", orderedJobIDs).
		Where("j.status IN ?", []int{entity.JobStatusQueued, entity.JobStatusWaitingGpu})

	var total int64
	if err := baseDB.Count(&total).Error; err != nil {
		return []response.QueueJobDBRow{}, 0, err
	}

	// 按队列中的顺序排序
	// MySQL FIND_IN_SET 或者 CASE WHEN，这里使用 CASE WHEN
	orderSQL := "CASE j.id"
	for i, id := range orderedJobIDs {
		orderSQL += fmt.Sprintf(" WHEN %d THEN %d", id, i)
	}
	orderSQL += " END"

	var rows []response.QueueJobDBRow
	offset := (req.Page - 1) * req.PageSize
	err := baseDB.
		Order(orderSQL).
		Limit(req.PageSize).Offset(offset).Scan(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	return rows, int(total), nil
}

// GetRunningJobs 获取正在执行中的任务列表
func (r *jobRepository) GetRunningJobs(req request.QueueListRequest, startTime, endTime time.Time) ([]response.QueueJobDBRow, int, error) {
	baseDB := r.buildBaseQueueJobQuery(req, startTime, endTime).
		Where("j.status = ?", entity.JobStatusRunning) // 只查询正在执行中的任务

	var total int64
	if err := baseDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []response.QueueJobDBRow
	err := baseDB.
		Order("j.created_at DESC"). // 正在执行中的任务按创建时间倒序排列
		Scan(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	return rows, int(total), nil
}

// buildBaseQueueJobQuery 辅助方法：构建队列任务的基础查询
func (r *jobRepository) buildBaseQueueJobQuery(req request.QueueListRequest, startTime, endTime time.Time) *gorm.DB {
	baseDB := r.db.Table("jobs j").
		Select(`j.id AS job_id,j.name AS job_name,j.description,j.status,j.sug,j.created_at AS submitted_at,u.username AS user_name`).
		Joins("LEFT JOIN admin_users u ON u.id = j.user_id")

	if req.Id > 0 {
		baseDB = baseDB.Where("j.id = ?", req.Id)
	}
	if req.Name != "" {
		baseDB = baseDB.Where("j.name LIKE ?", "%"+req.Name+"%")
	}
	if !startTime.IsZero() {
		baseDB = baseDB.Where("j.created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		baseDB = baseDB.Where("j.created_at <= ?", endTime)
	}
	return baseDB
}

// GetRunningJobsWithoutPagination 获取所有正在执行中的任务列表，不带分页
func (r *jobRepository) GetRunningJobsWithoutPagination(ctx context.Context) ([]*entity.Job, error) {
	var jobs []*entity.Job
	err := r.db.WithContext(ctx).Where("status = ?", entity.JobStatusRunning).Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// UpdateSug 修改标识
func (r *jobRepository) UpdateSug(jobId int) error {
	err := r.db.Table("jobs").Where("id = ?", jobId).Update("sug", 1).Error
	return err
}

func (r *jobRepository) GetStats() (*response.JobStatsResponse, error) {
	var stats response.JobStatsResponse
	err := r.db.Model(&entity.Job{}).
		Select("COUNT(CASE WHEN status = ? THEN 1 END) as pending, "+
			"COUNT(CASE WHEN status = ? THEN 1 END) as queued, "+
			"COUNT(CASE WHEN status = ? THEN 1 END) as running, "+
			"COUNT(CASE WHEN status = ? THEN 1 END) as completed, "+
			"COUNT(CASE WHEN status = ? THEN 1 END) as failed, "+
			"COUNT(CASE WHEN status = ? THEN 1 END) as cancelled",
			entity.JobStatusPending, entity.JobStatusQueued, entity.JobStatusRunning,
			entity.JobStatusCompleted, entity.JobStatusFailed, entity.JobStatusCancelled).
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}
