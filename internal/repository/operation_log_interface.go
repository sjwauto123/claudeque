package repository

import "cloudque/internal/model/entity"

type OperationLogRepository interface {
	Create(log *entity.OperationLog) error
}
