package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"

	"gorm.io/gorm"
)

type adminOperationLogRepository struct {
	db *gorm.DB
}

func NewAdminOperationLogRepository(db *gorm.DB) AdminOperationLogRepository {
	return &adminOperationLogRepository{db: db}
}

func (a *adminOperationLogRepository) FindAdminLogs(offset int, size int, keyWord string) (*[]dto.AdminLogResponse, int64, error) {
	// 使用 EXISTS 子查询限制查询范围为最新的5000条记录
	query := a.db.Model(&entity.AdminOperationLog{}).
		Where("EXISTS (SELECT 1 FROM (SELECT id FROM admin_operation_log ORDER BY created_at DESC LIMIT 5000) AS latest WHERE latest.id = admin_operation_log.id)")

	if keyWord != "" {
		query = query.Where("username LIKE ? OR action_type LIKE ? OR object LIKE  ?", "%"+keyWord+"%", "%"+keyWord+"%", "%"+keyWord+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []dto.AdminLogResponse
	if err := query.Offset(offset).Limit(size).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return &logs, total, nil
}

func (a *adminOperationLogRepository) CreateLog(log *entity.AdminOperationLog) (int, error) {
	if err := a.db.Create(log).Error; err != nil {
		return 0, err
	}
	return log.ID, nil
}

func (a *adminOperationLogRepository) UpdateStatus(id int) (int, error) {
	result := a.db.Model(&entity.AdminOperationLog{}).Where("id = ?", id).Update("status", 1)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}
