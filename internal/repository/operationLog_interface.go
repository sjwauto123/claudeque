package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"
)

type OperationLogRepository interface {
	FindAdminLogs(offset int, size int) (*[]dto.AdminLogResponse, int64, error)
	FindUserLogs(offset int, size int, username string, actionType string, start time.Time, end time.Time) (*[]dto.UserLogsResponse, int64, error)

	CreateLog(log *entity.OperationLog) error
	LimitLogs(limit int64) error
}
