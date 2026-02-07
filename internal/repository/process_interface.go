package repository

import "cloudque/internal/model/entity"

type ProcessRepository interface {
	Create(p *entity.Process) error
	DeleteByJobID(jobID int) error
	DeleteByPID(pid int) error
}
