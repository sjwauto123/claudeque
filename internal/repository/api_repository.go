package repository

import (
	"cloudque/internal/model/entity"
	"errors"
	"gorm.io/gorm"
)

type apiRepository struct {
	db *gorm.DB
}

// NewAPIRepository 创建API仓库实例
func NewAPIRepository(db *gorm.DB) APIRepository {
	return &apiRepository{db: db}
}

// GetAPIByID 根据ID获取API详情
func (r *apiRepository) GetAPIByID(id int) (*entity.Permission, error) {
	var api entity.Permission
	err := r.db.First(&api, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

// PageList 分页查询API列表
func (r *apiRepository) PageList(offset, limit int, httpPath string, status *int) ([]*entity.Permission, int64, error) {
	var apis []*entity.Permission
	var total int64

	// 构建查询条件
	query := r.db.Model(&entity.Permission{})

	// HTTP路径过滤
	if httpPath != "" {
		query = query.Where("http_path LIKE ?", "%"+httpPath+"%")
	}

	// 状态过滤
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 获取符合条件的总记录数
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 获取分页数据，按序号排序
	err = query.Order("sort").Offset(offset).Limit(limit).Find(&apis).Error
	if err != nil {
		return nil, 0, err
	}

	return apis, total, nil
}

// Create 创建API
func (r *apiRepository) Create(api *entity.Permission) error {
	return r.db.Create(api).Error
}

// Update 更新API
func (r *apiRepository) Update(api *entity.Permission) error {
	// 只更新非零值字段，避免更新created_at等字段
	updates := make(map[string]interface{})

	if api.Name != "" {
		updates["name"] = api.Name
	}
	if api.Category != "" {
		updates["category"] = api.Category
	}
	if api.Slug != "" {
		updates["slug"] = api.Slug
	}
	if api.Status != 0 {
		updates["status"] = api.Status
	}
	if api.HTTPMethod != "" {
		updates["http_method"] = api.HTTPMethod
	}
	if api.HTTPPath != "" {
		updates["http_path"] = api.HTTPPath
	}
	if api.Sort != 0 {
		updates["sort"] = api.Sort
	}

	return r.db.Model(api).Updates(updates).Error
}

func (r *apiRepository) Delete(id int) error {
	result := r.db.Delete(&entity.Permission{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// BatchDelete 批量删除API
func (r *apiRepository) BatchDelete(ids []int) error {
	result := r.db.Where("id IN ?", ids).Delete(&entity.Permission{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ExistsByName 判断API名称是否存在
func (r *apiRepository) ExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Permission{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsBySlug 判断API标识是否存在
func (r *apiRepository) ExistsBySlug(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Permission{}).Where("slug = ?", slug).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByHTTPPath 判断HTTP路径是否存在
func (r *apiRepository) ExistsByHTTPPath(httpPath string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Permission{}).Where("http_path = ?", httpPath).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
