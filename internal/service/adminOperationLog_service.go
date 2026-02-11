package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"cloudque/pkg/response"
)

type adminOperationLogService struct {
	adminOperationLogRep repository.AdminOperationLogRepository
}

func NewAdminOperationLogService(adminLogRep repository.AdminOperationLogRepository) AdminOperationLogService {
	return &adminOperationLogService{adminOperationLogRep: adminLogRep}
}

func (a *adminOperationLogService) GetAdminLogs(r *request.AdminLogsRequest) (interface{}, error) {
	offset := (r.Page - 1) * r.Size

	logs, total, err := a.adminOperationLogRep.FindAdminLogs(offset, r.Size, r.KeyWord)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalError, "查询管理员日志失败", err)
	}

	return response.NewPageResponse(logs, total, r.Page, r.Size), nil
}

func (a *adminOperationLogService) CreateLog(log *entity.AdminOperationLog) error {
	return a.adminOperationLogRep.CreateLog(log)
}
