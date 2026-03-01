package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"strings"
	"time"
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

func (s *menuService) PageList(req *request.MenuPageQueryRequest) ([]*dto.MenuTreeNode, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize

	parents, children, total, err := s.menuRepo.PageList(offset, req.PageSize, req.Title, req.Status)
	if err != nil {
		return nil, 0, err
	}

	// 组装树
	childMap := make(map[int][]*dto.MenuTreeNode)

	for _, c := range children {
		node := convertToNode(c)
		if c.ParentID != 0 {
			childMap[c.ParentID] = append(childMap[c.ParentID], node)
		}
	}

	var result []*dto.MenuTreeNode
	for _, p := range parents {
		parentNode := convertToNode(p)
		parentNode.Children = childMap[p.ID]
		result = append(result, parentNode)
	}

	return result, total, nil
}

func (s *menuService) Create(req *request.CreateMenuRequest) error {
	if req.Title == "" {
		return errors.New(errors.CodeMissingParam, "菜单名称不能为空")
	}
	if req.URI == "" {
		return errors.New(errors.CodeMissingParam, "菜单URI不能为空")
	}
	if req.Type == "" {
		return errors.New(errors.CodeMissingParam, "菜单类型不能为空")
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

	if *req.ParentID != 0 {
		req.Type = "menu"
	}

	menu := &entity.Menu{
		Title:    req.Title,
		Type:     req.Type,
		Status:   *req.Status,
		Icon:     req.Icon,
		URI:      req.URI,
		Sort:     req.Sort,
		ParentID: *req.ParentID,
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
	// 根据id校验菜单是否存在
	existingMenu, err := s.menuRepo.GetMenuByID(req.ID)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询菜单信息失败", err)
	}
	if existingMenu == nil {
		return errors.New(errors.CodeResourceNotFound, "菜单不存在")
	}

	updates := make(map[string]interface{})

	if req.Title != "" && req.Title != existingMenu.Title {
		exists, err := s.menuRepo.ExistsByTitleExcludingID(req.Title, req.ID)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "校验菜单标题失败", err)
		}
		if exists {
			return errors.New(errors.CodeResourceAlreadyExists, "菜单标题已存在")
		}
		updates["title"] = req.Title
	}

	if req.URI != "" && req.URI != existingMenu.URI {
		exists, err := s.menuRepo.ExistsByURIExcludingID(req.URI, req.ID)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "校验菜单 URI 失败", err)
		}
		if exists {
			return errors.New(errors.CodeResourceAlreadyExists, "菜单 URI 已存在")
		}
		updates["uri"] = req.URI
	}

	if req.Type != "" {
		updates["type"] = req.Type
	}

	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if req.Icon != "" {
		updates["icon"] = req.Icon
	}

	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.ParentID != 0 {
		updates["parent_id"] = req.ParentID
	}
	if len(updates) == 0 {
		return nil
	}

	// 设置更新时间
	updates["updated_at"] = time.Now()

	// 3. 执行更新
	if err := s.menuRepo.Update(req.ID, updates); err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "更新菜单失败", err)
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
		if m.ParentID == 0 {
			roots = append(roots, node)
		} else if parentNode, exists := menuMap[m.ParentID]; exists {
			parentNode.Children = append(parentNode.Children, node)
		}
	}

	return roots, nil
}
func convertToNode(m *entity.Menu) *dto.MenuTreeNode {
	return &dto.MenuTreeNode{
		ID:       m.ID,
		ParentID: m.ParentID,
		Title:    m.Title,
		Type:     m.Type,
		Status:   m.Status,
		Icon:     m.Icon,
		URI:      m.URI,
		Sort:     m.Sort,
		Children: []*dto.MenuTreeNode{},
	}
}
