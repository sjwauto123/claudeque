package service

import (
	"cloudque/pkg/response"
)

type AdminLogService interface {
	GetAdminLogs(r *response.PageRequest) (interface{}, error)
	LimitLogs(limit int64) error
}
