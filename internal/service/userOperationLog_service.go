package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"cloudque/pkg/response"
	"time"
)

type userOperationLogService struct {
	userOperationLogRep repository.UserOperationLogRepository
}

func NewUserOperationLogService(userLogRep repository.UserOperationLogRepository) UserOperationLogService {
	return &userOperationLogService{userOperationLogRep: userLogRep}
}
func (u *userOperationLogService) GetUserLogs(r *request.UserLogsRequest, start time.Time, end time.Time) (interface{}, error) {
	// 计算偏移量
	offset := (r.Page - 1) * r.Size

	// 查询日志
	logs, total, err := u.userOperationLogRep.FindUserLogs(offset, r.Size, r.Username, r.ActionType, r.Status, start, end)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalError, "查询用户日志失败", err)
	}

	// 创建分页响应
	return response.NewPageResponse(logs, total, r.Page, r.Size), nil
}

// CreateLog 创建操作日志
func (u *userOperationLogService) CreateLog(log *entity.UserOperationLog) error {
	return u.userOperationLogRep.CreateLog(log)
}
