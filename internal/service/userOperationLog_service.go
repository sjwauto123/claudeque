package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
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
		return nil, err
	}

	// 创建分页响应
	return response.NewPageResponse(logs, total, r.Page, r.Size), nil
}

// LimitLogs 限制日志数量
func (u *userOperationLogService) LimitLogs(limit int64) error {
	return u.userOperationLogRep.LimitLogs(limit)
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
	return u.userOperationLogRep.CreateLog(log)
}
