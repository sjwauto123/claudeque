package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
)

type APIService interface {
	// GetAPIByID 根据ID获取API详情
	GetAPIByID(id int) (*entity.Permission, error)

	// PageList 分页查询API列表
	PageList(req *request.APIPageQueryRequest) ([]*entity.Permission, int64, error)

	// Create 创建API
	Create(req *request.CreateAPIRequest) error

	// Update 更新API
	Update(req *request.UpdateAPIRequest) error

	// Delete 根据ID删除API
	Delete(id int) error

	// BatchDelete 批量删除API
	BatchDelete(ids []int) error
}
