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

// Create 创建进程信息
func (r *processRepository) Create(p *entity.Process) error {
	return r.db.Create(p).Error
}

// DeleteByJobID 根据任务id删除进程
func (r *processRepository) DeleteByJobID(jobID int) error {
	return r.db.Where("job_id = ?", jobID).Delete(&entity.Process{}).Error
}

// DeleteByPID 根据Pid删除进程
func (r *processRepository) DeleteByPID(pid int) error {
	return r.db.Where("pid = ?", pid).Delete(&entity.Process{}).Error
}
