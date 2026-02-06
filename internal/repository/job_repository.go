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
func (r jobRepository) GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID uint) ([]response.JobResponse, int64, int, int, error) {
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
		return nil, 0, req.Page, req.PageSize, err
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
		return nil, 0, req.Page, req.PageSize, err
	}

	return list, total, req.Page, req.PageSize, nil
}

// Create 创建任务
func (r jobRepository) Create(job *entity.Job) error {
	if err := r.db.Create(job).Error; err != nil {
		return err
	}
	return nil
}

// GetByID 根据ID获取任务
func (r jobRepository) GetByID(id uint) (*entity.Job, error) {
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
func (r jobRepository) UpdateStatus(id uint, status int) error {
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

// UpdateLogPath 更新任务日志路径
func (r jobRepository) UpdateLogPath(id uint, logPath string) error {
	return r.db.Model(&entity.Job{}).Where("id = ?", id).Update("log_path", logPath).Error
}

// GetQueueJobsByIDs 通过任务id获取任务信息
func (r jobRepository) GetQueueJobsByIDs(jobIDs []uint) (map[uint]response.QueueJobDBRow, error) {
	if len(jobIDs) == 0 {
		return map[uint]response.QueueJobDBRow{}, nil
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

	result := make(map[uint]response.QueueJobDBRow, len(rows))
	for _, r := range rows {
		result[r.JobID] = r
	}

	return result, nil
}

// GetQueueJobListFiltered 按队列顺序、条件筛选、分页获取排队任务
func (r jobRepository) GetQueueJobListFiltered(orderedJobIDs []uint, req request.JobListRequest, startTime, endTime time.Time) ([]response.QueueJobDBRow, int64, error) {
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
		ids[i] = strconv.FormatUint(uint64(id), 10)
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

	return rows, total, nil
}

func (r jobRepository) GetStats() (*response.JobStatsResponse, error) {
	var result response.JobStatsResponse

	if err := r.db.Model(&entity.Job{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&entity.Job{}).Where("status = ?", entity.JobStatusRunning).Count(&result.Running).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&entity.Job{}).Where("status = ?", entity.JobStatusQueued).Count(&result.Queued).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&entity.Job{}).Where("status IN ?", []int{entity.JobStatusFailed, entity.JobStatusCancelled}).Count(&result.Exception).Error; err != nil {
		return nil, err
	}

	return &result, nil
}
