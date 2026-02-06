package repository

import "cloudque/internal/model/entity"

type ProcessRepository interface {
	FindAll() ([]entity.Process, error)
}
