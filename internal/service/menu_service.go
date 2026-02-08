package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"strings"
)

type menuService struct {
	menuRepo repository.MenuRepository
}

func NewMenuService(menuRepo repository.MenuRepository) MenuService {
	return &menuService{menuRepo: menuRepo}
}

func (s *menuService) GetMenuByID(id int) (*entity.Menu, error) {
	return s.menuRepo.GetMenuByID(id)
}
func (s *menuService) PageList(req *request.MenuPageQueryRequest) ([]*entity.Menu, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 10
	}
	offset := (req.Page - 1) * req.PageSize

	return s.menuRepo.PageList(offset, req.PageSize, req.Title, req.Status)
}

func (s *menuService) Create(req *request.CreateMenuRequest) error {
	if req.Title == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}
	if req.URI == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}
	if req.Type == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}

	exists, err := s.menuRepo.ExistsByTitle(req.Title)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查菜单标题是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "菜单标题已存在")
	}

	exists, err = s.menuRepo.ExistsByURI(req.URI)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查菜单路由是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "菜单路由已存在")
	}

	menu := &entity.Menu{
		Title:    req.Title,
		Type:     req.Type,
		Status:   *req.Status,
		Icon:     req.Icon,
		URI:      req.URI,
		Sort:     req.Sort,
		ParentID: req.ParentID,
	}

	err = s.menuRepo.Create(menu)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.NewDefault(errors.CodeResourceAlreadyExists)
		}
		return errors.NewWithErr(errors.CodeInternalError, "创建菜单失败!", err)
	}

	return nil
}

func (s *menuService) Update(req *request.UpdateMenuRequest) error {
	existingMenu, err := s.menuRepo.GetMenuByID(req.ID)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询菜单信息失败", err)
	}
	if existingMenu == nil {
		return errors.New(errors.CodeResourceNotFound, "菜单不存在")
	}

	menu := &entity.Menu{
		BaseEntity: entity.BaseEntity{
			ID: req.ID,
		},
	}

	if req.Title != "" {
		menu.Title = req.Title
	}
	if req.Type != "" {
		menu.Type = req.Type
	}
	if req.Status != nil {
		menu.Status = *req.Status
	}
	if req.Icon != "" {
		menu.Icon = req.Icon
	}
	if req.URI != "" {
		menu.URI = req.URI
	}
	if req.Sort != nil {
		menu.Sort = *req.Sort
	}
	if req.ParentID != nil {
		menu.ParentID = req.ParentID
	}

	if err := s.menuRepo.Update(menu); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.NewDefault(errors.CodeResourceAlreadyExists)
		}
		return errors.NewWithErr(errors.CodeInternalError, "更新菜单失败!", err)
	}

	return nil
}

func (s *menuService) Delete(id int) error {
	existingMenu, err := s.menuRepo.GetMenuByID(id)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询菜单信息失败", err)
	}
	if existingMenu == nil {
		return errors.New(errors.CodeResourceNotFound, "菜单不存在")
	}

	return s.menuRepo.Delete(id)
}

func (s *menuService) BatchDelete(ids []int) error {
	if len(ids) == 0 {
		return errors.NewDefault(errors.CodeMissingParam)
	}

	for _, id := range ids {
		existingMenu, err := s.menuRepo.GetMenuByID(id)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "查询菜单信息失败", err)
		}
		if existingMenu == nil {
			return errors.New(errors.CodeResourceNotFound, "菜单不存在")
		}
	}

	return s.menuRepo.BatchDelete(ids)
}

func (s *menuService) BuildMenuTree(menus []*entity.Menu) ([]*dto.MenuTreeNode, error) {
	if len(menus) == 0 {
		return []*dto.MenuTreeNode{}, nil
	}

	menuMap := make(map[int]*dto.MenuTreeNode)
	for _, m := range menus {
		node := &dto.MenuTreeNode{
			ID:       m.ID,
			Title:    m.Title,
			Type:     m.Type,
			Status:   m.Status,
			Icon:     m.Icon,
			URI:      m.URI,
			Sort:     m.Sort,
			Children: []*dto.MenuTreeNode{},
		}
		menuMap[m.ID] = node
	}

	var roots []*dto.MenuTreeNode
	for _, m := range menus {
		node := menuMap[m.ID]
		if m.ParentID == nil {
			roots = append(roots, node)
		} else if parentNode, exists := menuMap[*m.ParentID]; exists {
			parentNode.Children = append(parentNode.Children, node)
		}
	}

	return roots, nil
}
