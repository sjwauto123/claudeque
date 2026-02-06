package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"gorm.io/gorm"
	"time"
)

type userLogRepository struct {
	db *gorm.DB
}

func NewUserLogRepository(db *gorm.DB) UserLogRepository {
	return &userLogRepository{db: db}
}
func (u *userLogRepository) FindLogs(offset int, size int, username string, actionType string, start time.Time, end time.Time) (*[]dto.UserLogsResponse, int64, error) {
	query := u.db.Model(&entity.UserLog{})

	// 操作类型
	if actionType != "" {
		query = query.Where("action_type = ?", actionType)
	}

	// 用户名（模糊）
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
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
	if err := query.Offset(offset).Limit(size).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return &logs, total, nil
}

func (u *userLogRepository) CreateLog(log *entity.UserLog) error {
	return u.db.Create(log).Error
}
