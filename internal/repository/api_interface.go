package repository

import (
	"cloudque/internal/model/entity"
)

type APIRepository interface {
	// PageList 分页查询API列表
	PageList(offset, limit int, httpPath string, status *int) ([]*entity.Permission, int64, error)

	// Create 创建API
	Create(api *entity.Permission) error

	// Update 更新API
	Update(api *entity.Permission) error

	// Delete 根据ID删除API
	Delete(id int) error

	// BatchDelete 批量删除API
	BatchDelete(ids []int) error

	// GetAPIByID 根据ID获取API详情
	GetAPIByID(id int) (*entity.Permission, error)

	// ExistsByName 判断API名称是否存在
	ExistsByName(name string) (bool, error)

	// ExistsBySlug 判断API标识是否存在
	ExistsBySlug(slug string) (bool, error)

	// ExistsByHTTPPath 判断HTTP路径是否存在
	ExistsByHTTPPath(httpPath string) (bool, error)
}
