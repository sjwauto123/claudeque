package repository

import (
	"cloudque/internal/model/entity"
	"errors"
	"gorm.io/gorm"
)

type menuRepository struct {
	db *gorm.DB
}

func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &menuRepository{db: db}
}

func (r *menuRepository) GetMenuByID(id int) (*entity.Menu, error) {
	var menu entity.Menu
	err := r.db.First(&menu, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) PageList(offset, limit int, title string, status *int) (
	[]*entity.Menu, []*entity.Menu, int64, error) {

	var parents []*entity.Menu
	var children []*entity.Menu
	var total int64
	// 只查父菜单
	query := r.db.Model(&entity.Menu{}).
		Where("parent_id = ?", 0)

	if title != "" {
		query = query.Where("title LIKE ?", "%"+title+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	// 统计父级总数
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	// 分页查父级
	if err := query.Order("sort ASC").
		Offset(offset).
		Limit(limit).
		Find(&parents).Error; err != nil {
		return nil, nil, 0, err
	}
	// 查子菜单
	var parentIDs []int
	for _, p := range parents {
		parentIDs = append(parentIDs, p.ID)
	}

	if len(parentIDs) > 0 {
		if err := r.db.Where("parent_id IN ?", parentIDs).
			Order("sort ASC").
			Find(&children).Error; err != nil {
			return nil, nil, 0, err
		}
	}

	return parents, children, total, nil
}

func (r *menuRepository) Create(menu *entity.Menu) error {
	return r.db.Create(menu).Error
}

func (r *menuRepository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&entity.Menu{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *menuRepository) Delete(id int) error {
	result := r.db.Delete(&entity.Menu{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *menuRepository) BatchDelete(ids []int) error {
	result := r.db.Where("id IN ?", ids).Delete(&entity.Menu{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *menuRepository) ExistsByTitle(title string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Menu{}).
		Where("title = ?", title).
		Count(&count).Error
	return count > 0, err
}

func (r *menuRepository) ExistsByURI(uri string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Menu{}).
		Where("uri = ?", uri).
		Count(&count).Error
	return count > 0, err
}

func (r *menuRepository) ExistsByTitleExcludingID(title string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Menu{}).
		Where("title = ?", title).
		Where("id != ?", excludeID). // 排除自身数据
		Count(&count).Error
	return count > 0, err
}

func (r *menuRepository) ExistsByURIExcludingID(uri string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Menu{}).
		Where("uri = ?", uri).
		Where("id != ?", excludeID). // 排除自身数据
		Count(&count).Error
	return count > 0, err
}
