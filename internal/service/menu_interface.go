package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
)

type MenuService interface {
	// PageList 分页查询菜单列表
	PageList(req *request.MenuPageQueryRequest) ([]*dto.MenuTreeNode, int64, error)

	// Create 创建菜单
	Create(req *request.CreateMenuRequest) error

	// Update 更新菜单
	Update(req *request.UpdateMenuRequest) error

	// Delete 根据ID删除菜单
	Delete(id int) error

	// BatchDelete 批量删除菜单
	BatchDelete(ids []int) error

	// GetMenuByID 根据ID获取菜单详情
	GetMenuByID(id int) (*entity.Menu, error)

	// BuildMenuTree 构建菜单树结构
	BuildMenuTree(menus []*entity.Menu) ([]*dto.MenuTreeNode, error)
}
