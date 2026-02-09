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

func (r *menuRepository) PageList(offset, limit int, title string, status *int) ([]*entity.Menu, []*entity.Menu, int64, error) {
	var parents []*entity.Menu
	var children []*entity.Menu
	var total int64

	// 1️⃣ 只查父菜单
	query := r.db.Model(&entity.Menu{}).
		Where("parent_id IS NULL")

	if title != "" {
		query = query.Where("title LIKE ?", "%"+title+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 2️⃣ 统计父级总数
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	// 3️⃣ 分页查父级
	if err := query.Order("sort ASC").
		Offset(offset).
		Limit(limit).
		Find(&parents).Error; err != nil {
		return nil, nil, 0, err
	}

	// 4️⃣ 查子菜单
	if len(parents) > 0 {
		var parentIDs []int
		for _, p := range parents {
			parentIDs = append(parentIDs, p.ID)
		}

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

func (r *menuRepository) Update(menu *entity.Menu) error {
	updates := make(map[string]interface{})

	if menu.Title != "" {
		updates["title"] = menu.Title
	}
	if menu.Type != "" {
		updates["type"] = menu.Type
	}
	if menu.Status != 0 {
		updates["status"] = menu.Status
	}
	if menu.Icon != "" {
		updates["icon"] = menu.Icon
	}
	if menu.URI != "" {
		updates["uri"] = menu.URI
	}
	if menu.Sort != 0 {
		updates["sort"] = menu.Sort
	}
	if menu.ParentID != nil {
		updates["parent_id"] = menu.ParentID
	}

	return r.db.Model(menu).Updates(updates).Error
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

func (r *menuRepository) ExistsByTitle(title string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Menu{}).Where("title = ?", title).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *menuRepository) ExistsByURI(uri string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Menu{}).Where("uri = ?", uri).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
