package repository

import (
	"cloudque/internal/model/entity"
)

// MenuRepository 菜单仓库接口
type MenuRepository interface {
	GetMenuByID(id int) (*entity.Menu, error)
	PageList(offset, limit int, title string, status *int) ([]*entity.Menu, []*entity.Menu, int64, error)
	Create(menu *entity.Menu) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	BatchDelete(ids []int) error
	ExistsByTitle(title string) (bool, error)
	ExistsByURI(uri string) (bool, error)
}
