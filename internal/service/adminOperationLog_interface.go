package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
)

type AdminOperationLogService interface {
	GetAdminLogs(r *request.AdminLogsRequest) (interface{}, error)
	CreateLog(log *entity.AdminOperationLog) (int, error)

	UpdateStatus(id int) (int, error)
}
