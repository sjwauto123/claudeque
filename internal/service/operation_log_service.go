package service

import (
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
)

type operationLogService struct {
	repo repository.OperationLogRepository
}

func NewOperationLogService(repo repository.OperationLogRepository) OperationLogService {
	return &operationLogService{repo: repo}
}

func (s *operationLogService) Log(username, actionType, object, description string, success bool) error {
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
	return s.repo.Create(log)
}
