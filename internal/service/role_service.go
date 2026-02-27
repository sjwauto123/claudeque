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

type roleService struct {
	roleRepo repository.RoleRepository
}

func NewRoleService(roleRepo repository.RoleRepository) RoleService {
	return &roleService{roleRepo: roleRepo}
}
func (s *roleService) GetRoleByID(id int) (*entity.Role, error) {
	return s.roleRepo.GetRoleByID(id)
}

func (s *roleService) PageList(req *request.RolePageQueryRequest) ([]*entity.Role, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 10
	}
	offset := (req.Page - 1) * req.PageSize
	return s.roleRepo.PageList(offset, req.PageSize, req.Name, req.Status)
}

func (s *roleService) Create(req *request.CreateRoleRequest) error {
	// 1.判断角色名是否重复
	if req.Name == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}
	exists, err := s.roleRepo.ExistsByName(req.Name)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查角色名是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "角色已存在")
	}
	// 2.判断角色slug是否重复
	if req.Slug == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}
	exists, err = s.roleRepo.ExistsBySlug(req.Slug)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查角色标识是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "角色标识已存在")
	}
	// 3.保存到数据库
	role := &entity.Role{
		Name:   req.Name,
		Slug:   req.Slug,
		Status: *req.Status,
	}
	err = s.roleRepo.Create(role)
	if err != nil {
		// 是否唯一约束冲突(兜底方案)
		if strings.Contains(err.Error(), "Duplicate entry") ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.NewDefault(errors.CodeResourceAlreadyExists)
		}
		return errors.NewWithErr(errors.CodeInternalError, "创建角色失败!", err)
	}
	return nil
}

func (s *roleService) Update(req *request.UpdateRoleRequest) error {
	// 首先根据id查询该角色是否存在
	existing, err := s.roleRepo.GetRoleByID(req.ID)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询角色失败", err)
	}
	if existing == nil {
		return errors.New(errors.CodeResourceNotFound, "角色不存在")
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		exists, err := s.roleRepo.ExistsByName(req.Name)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "校验角色名失败", err)
		}
		if exists {
			return errors.New(errors.CodeResourceAlreadyExists, "角色名已存在")
		}
		updates["name"] = req.Name
	}

	if req.Slug != "" {
		exists, err := s.roleRepo.ExistsBySlug(req.Slug)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "校验角色标识失败", err)
		}
		if exists {
			return errors.New(errors.CodeResourceAlreadyExists, "角色标识已存在")
		}
		updates["slug"] = req.Slug
	}

	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) == 0 {
		return nil
	}

	updates["updated_at"] = time.Now()
	// 调用 repository
	err = s.roleRepo.Update(req.ID, updates)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "更新角色失败!", err)
	}
	return nil
}

func (s *roleService) Delete(id int) error {
	return s.roleRepo.Delete(id)
}
func (s *roleService) BatchDelete(ids []int) error {
	return s.roleRepo.BatchDelete(ids)
}

// GetRolePermissionByID 根据id获取角色权限
// func (s *roleService) GetRolePermissionByID(roleID int) ([]*dto.RolePermissionNodeRes, error) {
//
//		menus, permissions, permissionMenus, roleMenuMap, rolePermissionMap, err :=
//			s.roleRepo.GetRolePermissionByID(roleID)
//
//		if err != nil {
//			return nil, err
//		}
//
//		// 1. 构建菜单节点 map
//		menuNodeMap := make(map[int]*dto.RolePermissionNodeRes)
//
//		for _, m := range menus {
//			menuNodeMap[m.ID] = &dto.RolePermissionNodeRes{
//				ID:         m.ID,
//				Title:      m.Title,
//				Checked:    roleMenuMap[m.ID],
//				Permission: []*dto.PermissionNodeRes{},
//				Children:   []*dto.RolePermissionNodeRes{},
//			}
//		}
//
//		// 2. 构建 permission -> menuIDs 映射
//		permMenuMap := make(map[int][]int)
//
//		for _, pm := range permissionMenus {
//			permMenuMap[pm.PermissionID] = append(
//				permMenuMap[pm.PermissionID],
//				pm.MenuID,
//			)
//		}
//
//		// 3. 把权限挂到对应菜单
//		for _, p := range permissions {
//
//			permNode := &dto.PermissionNodeRes{
//				ID:      p.ID,
//				Name:    p.Name,
//				Slug:    p.Slug,
//				Checked: rolePermissionMap[p.ID],
//			}
//
//			menuIDs := permMenuMap[p.ID]
//
//			for _, menuID := range menuIDs {
//				if menuNode, ok := menuNodeMap[menuID]; ok {
//					menuNode.Permission = append(menuNode.Permission, permNode)
//				}
//			}
//		}
//
//		// 4. 构建树结构
//		var roots []*dto.RolePermissionNodeRes
//
//		for _, m := range menus {
//			node := menuNodeMap[m.ID]
//
//			if m.ParentID == 0 {
//				roots = append(roots, node)
//			} else {
//				if parent, ok := menuNodeMap[m.ParentID]; ok {
//					parent.Children = append(parent.Children, node)
//				}
//			}
//		}
//
//		return roots, nil
//	}
func (s *roleService) GetRolePermissionByID(roleID int) (*dto.RolePermissionResponse, error) {
	menus, permissions, roleMenuMap, rolePermMap, err :=
		s.roleRepo.GetRolePermissionByID(roleID)
	if err != nil {
		return nil, err
	}

	menuTree := s.buildMenuTree(menus, roleMenuMap)

	// 👇 新增：提取 category 顺序
	categoryOrder := s.extractCategoryOrder(menuTree)

	// 👇 修改：传入 categoryOrder
	apiGroups := s.buildApiPermissions(permissions, rolePermMap, categoryOrder)

	return &dto.RolePermissionResponse{
		Menu: menuTree,
		Api:  apiGroups,
	}, nil
}

// 构建菜单树
func (s *roleService) buildMenuTree(menus []entity.Menu, roleMenuMap map[int]bool) []*dto.MenuNodeResponse {
	nodeMap := make(map[int]*dto.MenuNodeResponse)

	for _, m := range menus {
		nodeMap[m.ID] = &dto.MenuNodeResponse{
			ID:      m.ID,
			Title:   m.Title,
			Checked: roleMenuMap[m.ID],
		}
	}
	var roots []*dto.MenuNodeResponse
	for _, m := range menus {
		node := nodeMap[m.ID]
		if m.ParentID == 0 {
			roots = append(roots, node)
		} else if parent, ok := nodeMap[m.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		}
	}
	return roots
}

// 构建权限api树
func (s *roleService) buildApiPermissions(
	perms []entity.Permission,
	rolePermMap map[int]bool,
	categoryOrder []string,
) []*dto.ApiPermissionGroupRes {

	groupMap := make(map[string][]*dto.ApiPermissionRes)

	for _, p := range perms {
		groupMap[p.Category] = append(groupMap[p.Category], &dto.ApiPermissionRes{
			ID:      p.ID,
			Name:    p.Name,
			Slug:    p.Slug,
			Checked: rolePermMap[p.ID],
		})
	}

	// 构建 category -> index 映射
	orderMap := make(map[string]int)
	for i, cat := range categoryOrder {
		orderMap[cat] = i
	}

	// 按 categoryOrder 的顺序构建结果
	res := make([]*dto.ApiPermissionGroupRes, 0, len(groupMap))

	for _, cat := range categoryOrder {
		if list, exists := groupMap[cat]; exists {
			res = append(res, &dto.ApiPermissionGroupRes{
				Category: cat,
				List:     list,
			})
			delete(groupMap, cat) // 避免重复
		}
	}

	// 处理不在菜单中的 category
	for cat, list := range groupMap {
		res = append(res, &dto.ApiPermissionGroupRes{
			Category: cat,
			List:     list,
		})
	}

	return res
}

func (s *roleService) UpdateRolePermission(roleID int, req *request.UpdateRolePermissionRequest) error {
	return s.roleRepo.UpdateRolePermission(roleID, req.MenuIDs, req.PermissionIDs)
}

// extractCategoryOrder 从菜单树中提取所有叶子节点的 Title（即 category 顺序）
func (s *roleService) extractCategoryOrder(menuTree []*dto.MenuNodeResponse) []string {
	var order []string
	for _, node := range menuTree {
		if len(node.Children) == 0 {
			// 一级叶子菜单
			order = append(order, node.Title)
		} else {
			// 有子菜单，遍历子菜单作为 category
			for _, child := range node.Children {
				order = append(order, child.Title)
			}
		}
	}
	return order
}
