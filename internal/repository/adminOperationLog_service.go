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
	query := a.db.Model(&entity.AdminOperationLog{}).Select("username, action_type, object, status, created_at")

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

func (a *adminOperationLogRepository) CreateLog(log *entity.AdminOperationLog) error {
	return a.db.Create(log).Error
}
