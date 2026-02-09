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
	userOperationLogRep repository.OperationLogRepository
}

func NewUserOperationLogService(userLogRep repository.OperationLogRepository) UserOperationLogService {
	return &userOperationLogService{userOperationLogRep: userLogRep}
}
func (u *userOperationLogService) GetUserLogs(r *request.UserLogsRequest, start time.Time, end time.Time) (interface{}, error) {
	// 计算偏移量
	offset := (r.Page - 1) * r.Size

	// 查询日志
	logs, total, err := u.userOperationLogRep.FindUserLogs(offset, r.Size, r.Username, r.ActionType, start, end)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalError, "查询用户日志失败", err)
	}

	// 创建分页响应
	return response.NewPageResponse(logs, total, r.Page, r.Size), nil
}

// LimitLogs 限制日志数量
func (u *userOperationLogService) LimitLogs(limit int64) error {
	err := u.userOperationLogRep.LimitLogs(limit)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "限制日志数量失败", err)
	}
	return nil
}

func (u *userOperationLogService) CreateLog(username, actionType, object, description string, success bool) error {
	status := 0
	if !success {
		status = 1
	}

	log := &entity.OperationLog{
		Username:    username,
		ActionType:  actionType,
		Object:      object,
		Description: description,
		Status:      status,
	}

	err := u.userOperationLogRep.CreateLog(log)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "创建操作日志失败", err)
	}
	return nil
}
