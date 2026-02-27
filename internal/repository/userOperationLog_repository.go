package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"

	"gorm.io/gorm"
)

type userOperationLogRepository struct {
	db *gorm.DB
}

func NewUserOperationLogRepository(db *gorm.DB) UserOperationLogRepository {
	return &userOperationLogRepository{db: db}
}

// FindUserLogs 查询非关机或重启类型的日志
func (u *userOperationLogRepository) FindUserLogs(offset int, size int, username string, actionType string, start time.Time, end time.Time) (*[]dto.UserLogsResponse, int64, error) {
	// 使用 JOIN 方式限制查询范围为最新的5000条记录，性能优于 IN 子查询
	query := u.db.Table("operation_logs AS l").
		Select("l.id, l.username, l.method, l.request_data, l.action_type, l.path, l.status, l.created_at, l.updated_at").
		Joins("INNER JOIN (SELECT id FROM operation_logs ORDER BY created_at DESC LIMIT 5000) AS latest ON l.id = latest.id")

	// 操作类型
	if actionType != "" {
		query = query.Where("l.action_type LIKE ?", "%"+actionType+"%")
	}

	// 用户名（模糊）
	if username != "" {
		query = query.Where("l.username LIKE ?", "%"+username+"%")
	}

	// 时间范围（灵活支持单边）
	if !start.Equal(time.Time{}) {
		query = query.Where("l.created_at >= ?", start)
	}

	if !end.Equal(time.Time{}) {
		query = query.Where("l.created_at <= ?", end)
	}

	// 总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页数据
	var logs []dto.UserLogsResponse
	if err := query.Offset(offset).Limit(size).Order("l.created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return &logs, total, nil
}

// CreateLog 创建操作日志
func (u *userOperationLogRepository) CreateLog(log *entity.UserOperationLog) error {

	return u.db.Create(log).Error
}
