package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"gorm.io/gorm"
	"time"
)

type operationLogRepository struct {
	db *gorm.DB
}

func NewOperationLogRepository(db *gorm.DB) OperationLogRepository {
	return &operationLogRepository{db: db}
}

func (u *operationLogRepository) CreateLog(log *entity.OperationLog) error {
	return u.db.Create(log).Error
}

// FindUserLogs 查询非关机或重启类型的日志
func (u *operationLogRepository) FindUserLogs(offset int, size int, username string, actionType string, start time.Time, end time.Time) (*[]dto.UserLogsResponse, int64, error) {
	query := u.db.Model(&entity.OperationLog{})

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

// LimitLogs 限制日志数量，只保留最近的limit条
func (u *operationLogRepository) LimitLogs(limit int64) error {
	// 计算需要删除的日志数量
	var total int64
	if err := u.db.Model(&entity.OperationLog{}).Count(&total).Error; err != nil {
		return err
	}

	if total > limit {
		// 获取需要保留的日志ID
		var keepIDs []int64
		if err := u.db.Model(&entity.OperationLog{}).
			Order("created_at DESC").
			Limit(int(limit)).
			Pluck("id", &keepIDs).Error; err != nil {
			return err
		}

		// 删除不在保留列表中的日志
		return u.db.Where("id NOT IN ?", keepIDs).Delete(&entity.OperationLog{}).Error
	}

	return nil
}
