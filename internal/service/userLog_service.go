package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/repository"
	"cloudque/pkg/response"
	"fmt"
	"time"
)

type userLogService struct {
	userLogRep repository.UserLogRepository
}

func NewUserLogService(userLogRep repository.UserLogRepository) UserLogService {
	return &userLogService{userLogRep: userLogRep}
}
func (u *userLogService) GetUserLogs(r *request.UserLogsRequest) (interface{}, error) {
	// 参数标准化
	page := r.Page
	if page < 1 {
		page = 1
	}
	size := r.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	// 计算偏移量
	offset := (page - 1) * size

	var start, end time.Time
	if r.StartTime != "" {
		t, err := time.Parse("2006-01-02", r.StartTime)
		if err != nil {
			return nil, fmt.Errorf("invalid startTime, expected YYYY-MM-DD")
		}
		start = t
	}

	if r.EndTime != "" {
		t, err := time.Parse("2006-01-02", r.EndTime)
		if err != nil {
			return nil, fmt.Errorf("invalid finial, expected YYYY-MM-DD")
		}
		// 结束时间扩展到当天 23:59:59
		t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		end = t
	}

	logs, total, err := u.userLogRep.FindLogs(offset, size, r.Username, r.ActionType, start, end)
	if err != nil {
		return nil, err
	}

	return response.NewPageResponse(logs, total, page, size), nil

}
