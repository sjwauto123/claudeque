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
func (u *userOperationLogRepository) FindUserLogs(offset int, size int, username string, actionType string, status string, start time.Time, end time.Time) (*[]dto.UserLogsResponse, int64, error) {
	// 使用 EXISTS 子查询限制查询范围为最新的5000条记录
	query := u.db.Model(&entity.UserOperationLog{}).
		Where("EXISTS (SELECT 1 FROM (SELECT id FROM operation_logs ORDER BY created_at DESC LIMIT 5000) AS latest WHERE latest.id = operation_logs.id)")

	// 操作类型
	if actionType != "" {
		query = query.Where("action_type LIKE ?", "%"+actionType+"%")
	}

	// 用户名（模糊）
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}

	// 状态筛选
	if status == "success" {
		query = query.Where("status = ? ", 200)
	} else if status == "fail" {
		query = query.Where("status != ? ", 200)
	}

	// 时间范围（灵活支持单边）
	if !start.Equal(time.Time{}) {
		query = query.Where("created_at >= ?", start)
	}

	if !end.Equal(time.Time{}) {
		query = query.Where("created_at <= ?", end)
	}

	// 总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页数据
	var logs []dto.UserLogsResponse
	if err := query.Offset(offset).Limit(size).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return &logs, total, nil
}

// CreateLog 创建操作日志
func (u *userOperationLogRepository) CreateLog(log *entity.UserOperationLog) error {
	return u.db.Create(log).Error
}
