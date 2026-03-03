package repository

import (
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
)

type AdminOperationLogRepository interface {
	FindAdminLogs(offset int, size int, keyWord string, status string) (*[]dto.AdminLogResponse, int64, error)
	CreateLog(log *entity.AdminOperationLog) (int, error)

	UpdateStatus(id int) (int, error)
}
