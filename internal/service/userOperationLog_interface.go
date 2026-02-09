package service

import (
	"cloudque/internal/model/dto/request"
	"time"
)

type UserOperationLogService interface {
	GetUserLogs(r *request.UserLogsRequest, start time.Time, end time.Time) (interface{}, error)

	CreateLog(username, actionType, object, description string, success bool) error

	LimitLogs(limit int64) error
}
