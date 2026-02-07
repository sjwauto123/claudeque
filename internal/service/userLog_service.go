package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/repository"
	"cloudque/pkg/response"
	"time"
)

type userLogService struct {
	userLogRep repository.OperationLogRepository
}

func NewUserLogService(userLogRep repository.OperationLogRepository) UserLogService {
	return &userLogService{userLogRep: userLogRep}
}
func (u *userLogService) GetUserLogs(r *request.UserLogsRequest, start time.Time, end time.Time) (interface{}, error) {
	// 计算偏移量
	offset := (r.Page - 1) * r.Size

	// 查询非关机和非重启类型的日志
	logs, total, err := u.userLogRep.FindUserLogs(offset, r.Size, r.Username, r.ActionType, start, end)
	if err != nil {
		return nil, err
	}

	// 创建分页响应
	return response.NewPageResponse(logs, total, r.Page, r.Size), nil
}
