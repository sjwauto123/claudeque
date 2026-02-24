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
	// 先获取最新的5000条记录的ID
	var latestIDs []int
	if err := a.db.Model(&entity.AdminOperationLog{}).
		Select("id").
		Order("created_at DESC").
		Limit(5000).
		Pluck("id", &latestIDs).Error; err != nil {
		return nil, 0, err
	}

	if len(latestIDs) == 0 {
		return &[]dto.AdminLogResponse{}, 0, nil
	}

	query := a.db.Model(&entity.AdminOperationLog{}).Select("username, action_type, object, status, created_at").Where("id IN ?", latestIDs)

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
