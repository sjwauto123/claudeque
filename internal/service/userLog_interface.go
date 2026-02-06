package service

import "cloudque/internal/model/dto/request"

type UserLogService interface {
	GetUserLogs(r *request.UserLogsRequest) (interface{}, error)
}
