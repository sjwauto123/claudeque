package repository

import (
	"cloudque/internal/model/entity"
)

// MenuRepository 菜单仓库接口
type MenuRepository interface {
	PageList(offset, limit int, title string, status *int) ([]*entity.Menu, int64, error)
	Create(menu *entity.Menu) error
	Update(menu *entity.Menu) error
	Delete(id int) error
	BatchDelete(ids []int) error
	GetMenuByID(id int) (*entity.Menu, error)
	ExistsByTitle(title string) (bool, error)
	ExistsByURI(uri string) (bool, error)
}
