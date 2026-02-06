package service

import (
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/logger"

	"go.uber.org/zap"
)

// LogService 日志服务接口
type LogService interface {
	CreateLog(username string, actionType, description string)
	CreateRequestLog(log *entity.RequestLog)
}

// logService 日志服务实现
type logService struct {
	logRepo repository.LogRepository
}

// NewLogService 创建日志服务
func NewLogService(logRepo repository.LogRepository) LogService {
	return &logService{
		logRepo: logRepo,
	}
}

// CreateLog 创建日志 (异步记录，不阻塞主流程)
func (s *logService) CreateLog(username string, actionType, description string) {
	go func() {
		log := &entity.OperationLog{
			Username:    username,
			ActionType:  actionType,
			Description: description,
		}
		if err := s.logRepo.Create(log); err != nil {
			logger.Error("记录操作日志失败",
				zap.String("username", username),
				zap.String("action", actionType),
				zap.Error(err),
			)
		}
	}()
}

// CreateRequestLog 创建请求日志 (异步)
func (s *logService) CreateRequestLog(log *entity.RequestLog) {
	go func() {
		if err := s.logRepo.CreateRequestLog(log); err != nil {
			logger.Error("记录请求日志失败",
				zap.String("path", log.Path),
				zap.Error(err),
			)
		}
	}()
}
