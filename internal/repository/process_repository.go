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

func (r *processRepository) FindAll() ([]entity.Process, error) {
	var processes []entity.Process
	err := r.db.Find(&processes).Error
	return processes, err
}
