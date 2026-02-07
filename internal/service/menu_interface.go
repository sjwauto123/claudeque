package service

import (
	"cloudque/internal/model/dto/request"
	response2 "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
)

type MenuService interface {
	// PageList 分页查询菜单列表
	PageList(req *request.MenuPageQueryRequest) ([]*entity.Menu, int64, error)

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
	BuildMenuTree(menus []*entity.Menu) ([]*response2.MenuTreeNode, error)
}
