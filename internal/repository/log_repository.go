package repository

import (
	"cloudque/internal/model/entity"
	"gorm.io/gorm"
)

// LogRepository 日志仓库接口
type LogRepository interface {
	Create(log *entity.OperationLog) error
	CreateRequestLog(log *entity.RequestLog) error
}

// logRepository 日志仓库实现
type logRepository struct {
	db *gorm.DB
}

// NewLogRepository 创建日志仓库
func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

// Create 创建日志
func (r *logRepository) Create(log *entity.OperationLog) error {
	return r.db.Create(log).Error
}

// CreateRequestLog 创建请求日志
func (r *logRepository) CreateRequestLog(log *entity.RequestLog) error {
	return r.db.Create(log).Error
}
