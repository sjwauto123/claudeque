package service

import (
	"cloudque/internal/model/dto/request"
	"time"
)

type UserLogService interface {
	GetUserLogs(r *request.UserLogsRequest, start time.Time, end time.Time) (interface{}, error)
}
