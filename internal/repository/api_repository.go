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
	err = query.Order("category").Order("sort").Offset(offset).Limit(limit).Find(&apis).Error
	if err != nil {
		return nil, 0, err
	}

	return apis, total, nil
}

func (r *apiRepository) Create(api *entity.Permission) error {
	return r.db.Create(api).Error
}

func (r *apiRepository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&entity.Permission{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *apiRepository) Delete(id int) error {
	return r.db.Delete(&entity.Permission{}, id).Error
}

// BatchDelete 批量删除API
func (r *apiRepository) BatchDelete(ids []int) error {
	return r.db.Where("id IN ?", ids).Delete(&entity.Permission{}).Error
}

// ExistsBySlug 判断API标识是否存在
func (r *apiRepository) ExistsBySlug(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Permission{}).
		Where("slug = ?", slug).
		Count(&count).Error
	return count > 0, err
}

func (r *apiRepository) ExistsBySlugExcludingID(slug string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Permission{}).
		Where("slug = ?", slug).
		Where("id != ?", excludeID). // 排除自身数据
		Count(&count).Error
	return count > 0, err
}

// ExistsByMethodAndPath 检查 (http_method, http_path) 是否已存在
func (r *apiRepository) ExistsByMethodAndPath(method, path string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Permission{}).
		Where("http_method = ? AND http_path = ?", method, path).
		Count(&count).Error
	return count > 0, err
}
