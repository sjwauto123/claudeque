package service

import (
	"cloudque/internal/repository"
	"cloudque/pkg/response"
)

type adminLogService struct {
	adminLogRep repository.OperationLogRepository
}

func NewAdminLogService(adminLogRep repository.OperationLogRepository) AdminLogService {
	return &adminLogService{
		adminLogRep: adminLogRep,
	}
}

func (a *adminLogService) GetAdminLogs(r *response.PageRequest) (interface{}, error) {
	// 计算偏移量
	offset := (r.Page - 1) * r.Size

	// 查询关机或重启类型的日志
	logs, total, err := a.adminLogRep.FindAdminLogs(offset, r.Size)
	if err != nil {
		return nil, err
	}

	// 创建分页响应
	return response.NewPageResponse(logs, total, r.Page, r.Size), nil
}

// LimitLogs 限制日志数量
func (a *adminLogService) LimitLogs(limit int64) error {
	return a.adminLogRep.LimitLogs(limit)
}
