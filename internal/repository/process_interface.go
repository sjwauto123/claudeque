package repository

import "cloudque/internal/model/entity"

type ProcessRepository interface {
	// Create 创建进程信息
	Create(p *entity.Process) error
	// DeleteByJobID 根据任务id删除进程
	DeleteByJobID(jobID int) error
	// DeleteByPID 根据Pid删除进程
	DeleteByPID(pid int) error
	// FindAll 获得所有进程
	FindAll() ([]entity.Process, error)
}
