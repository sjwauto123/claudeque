package repository

import (
	"errors"
	"strconv"
	"strings"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
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
func (r jobRepository) GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int, int, int, error) {
	var (
		list  []response.JobResponse
		total int64
	)

	baseDB := r.db.Table("jobs j").
		Where("j.user_id = ?", userID)

	// 条件
	if req.Id > 0 {
		baseDB = baseDB.Where("j.id = ?", req.Id)
	}
	if req.Status > 0 {
		baseDB = baseDB.Where("j.status = ?", req.Status)
	} else {
		baseDB = baseDB.Where("j.status = ?", 1)
		baseDB = baseDB.Where("j.status = ?", 6)
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
		Select(`
			j.id,
			j.name,
			j.description,
			j.status,
			j.created_at,
			COALESCE(g.card, '') AS card,
			0 AS count
		`).
		Joins(`
			LEFT JOIN (
				SELECT current_job_id,
					   GROUP_CONCAT(name ORDER BY id SEPARATOR ',') AS card
				FROM gpu_cards
				GROUP BY current_job_id
			) g ON g.current_job_id = j.id
		`).
		Order("j.created_at DESC").
		Limit(req.PageSize).
		Offset(offset).
		Scan(&list).Error

	if err != nil {
		return []response.JobResponse{}, 0, req.Page, req.PageSize, err
	}

	return list, int(total), req.Page, req.PageSize, nil
}

// Create 创建任务
func (r jobRepository) Create(job *entity.Job) error {
	if err := r.db.Create(job).Error; err != nil {
		return err
	}
	return nil
}

// GetByID 根据ID获取任务
func (r jobRepository) GetByID(id int) (*entity.Job, error) {
	var job entity.Job
	if err := r.db.First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

// UpdateStatus 更新任务状态
func (r jobRepository) UpdateStatus(id int, status int) error {
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
func (r jobRepository) GetQueueJobsByIDs(jobIDs []int) (map[int]response.QueueJobDBRow, error) {
	if len(jobIDs) == 0 {
		return map[int]response.QueueJobDBRow{}, nil
	}

	var rows []response.QueueJobDBRow

	err := r.db.Table("jobs j").
		Select(`
			j.id AS job_id,
			j.name AS job_name,
			j.description,
			j.status,
			j.created_at AS submitted_at,
			u.username AS user_name
		`).
		Joins("LEFT JOIN users u ON u.id = j.user_id").
		Where("j.id IN ?", jobIDs).
		Scan(&rows).Error

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
func (r jobRepository) GetQueueJobListFiltered(orderedJobIDs []int, req request.JobListRequest, startTime, endTime time.Time) ([]response.QueueJobDBRow, int, error) {
	if len(orderedJobIDs) == 0 {
		return []response.QueueJobDBRow{}, 0, nil
	}

	baseDB := r.db.Table("jobs j").
		Select(`
			j.id AS job_id,
			j.name AS job_name,
			j.description,
			j.status,
			j.created_at AS submitted_at,
			u.username AS user_name
		`).
		Joins("LEFT JOIN users u ON u.id = j.user_id").
		Where("j.id IN ?", orderedJobIDs)

	if req.Id > 0 {
		baseDB = baseDB.Where("j.id = ?", req.Id)
	}
	if req.Status > 0 {
		baseDB = baseDB.Where("j.status = ?", req.Status)
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

	var total int64
	if err := baseDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (req.Page - 1) * req.PageSize

	ids := make([]string, len(orderedJobIDs))
	for i, id := range orderedJobIDs {
		ids[i] = strconv.Itoa(id)
	}

	orderSQL := "FIELD(j.id, " + strings.Join(ids, ",") + ")"
	var rows []response.QueueJobDBRow
	err := baseDB.
		Order(orderSQL).
		Limit(req.PageSize).
		Offset(offset).
		Scan(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	return rows, int(total), nil
}

// GetStats 获取任务统计
func (r jobRepository) GetStats() (*response.JobStatsResponse, error) {
	var result response.JobStatsResponse
	var total, running, queued, exception int64

	if err := r.db.Model(&entity.Job{}).Count(&total).Error; err != nil {
		return nil, err
	}
	result.Total = int(total)

	if err := r.db.Model(&entity.Job{}).Where("status = ?", entity.JobStatusRunning).Count(&running).Error; err != nil {
		return nil, err
	}
	result.Running = int(running)

	if err := r.db.Model(&entity.Job{}).Where("status = ?", entity.JobStatusQueued).Count(&queued).Error; err != nil {
		return nil, err
	}
	result.Queued = int(queued)

	if err := r.db.Model(&entity.Job{}).Where("status IN ?", []int{entity.JobStatusFailed, entity.JobStatusCancelled}).Count(&exception).Error; err != nil {
		return nil, err
	}
	result.Exception = int(exception)

	return &result, nil
}
