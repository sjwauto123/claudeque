package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"
)

type UserLogRepository interface {
	FindLogs(offset int, size int, username string, actionType string, start time.Time, end time.Time) (*[]dto.UserLogsResponse, int64, error)

	CreateLog(log *entity.UserLog) error
}
