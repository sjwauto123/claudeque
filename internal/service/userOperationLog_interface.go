package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"time"
)

type UserOperationLogService interface {
	GetUserLogs(r *request.UserLogsRequest, start time.Time, end time.Time) (interface{}, error)
	CreateLog(log *entity.UserOperationLog) error
}
