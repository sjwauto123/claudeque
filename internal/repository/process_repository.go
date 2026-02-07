package repository

import (
	"cloudque/internal/model/entity"
	"gorm.io/gorm"
)

type processRepository struct {
	db *gorm.DB
}

func NewProcessRepository(db *gorm.DB) ProcessRepository {
	return &processRepository{db: db}
}

func (r *processRepository) Create(p *entity.Process) error {
	return r.db.Create(p).Error
}

func (r *processRepository) DeleteByJobID(jobID int) error {
	return r.db.Where("job_id = ?", jobID).Delete(&entity.Process{}).Error
}

func (r *processRepository) DeleteByPID(pid int) error {
	return r.db.Where("pid = ?", pid).Delete(&entity.Process{}).Error
}
