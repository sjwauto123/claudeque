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
	// ExistsByTitle 判断菜单名称是否存在
	ExistsByTitle(title string) (bool, error)
	// ExistsByURI 判断菜单URI是否存在
	ExistsByURI(uri string) (bool, error)
	// ExistsByTitleExcludingID 排除检查自身数据
	ExistsByTitleExcludingID(title string, excludeID int) (bool, error)
	// ExistsByURIExcludingID 排除检查自身数据
	ExistsByURIExcludingID(uri string, excludeID int) (bool, error)
}
