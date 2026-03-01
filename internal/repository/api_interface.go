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
	Update(id int, updates map[string]interface{}) error

	// Delete 根据ID删除API
	Delete(id int) error

	// BatchDelete 批量删除API
	BatchDelete(ids []int) error

	// GetAPIByID 根据ID获取API详情
	GetAPIByID(id int) (*entity.Permission, error)

	// ExistsBySlug 判断API标识是否存在
	ExistsBySlug(slug string) (bool, error)

	// ExistsByMethodAndPath 检查 (http_method, http_path) 是否已存在
	ExistsByMethodAndPath(method, path string) (bool, error)

	// ExistsBySlugExcludingID 排除检查自身数据
	ExistsBySlugExcludingID(slug string, excludeID int) (bool, error)
}
