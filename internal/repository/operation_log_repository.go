package repository

import (
	"cloudque/internal/model/entity"
	"gorm.io/gorm"
)

type operationLogRepository struct {
	db *gorm.DB
}

func NewOperationLogRepository(db *gorm.DB) OperationLogRepository {
	return &operationLogRepository{db: db}
}

func (r *operationLogRepository) Create(log *entity.OperationLog) error {
	return r.db.Create(log).Error
}
