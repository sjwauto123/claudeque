package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"strconv"
	"strings"
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
	role := &entity.Role{
		BaseEntity: entity.BaseEntity{
			ID: req.ID,
		},
		Name:   req.Name,
		Slug:   req.Slug,
		Status: *req.Status,
	}
	if req.Status != nil {
		role.Status = *req.Status
	}

	if err := s.roleRepo.Update(role); err != nil {
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
func (s *roleService) GetRolePermissionByID(roleID int) (*dto.RolePermissionTree, error) {
	// Step 1: 从 Repository 获取原始数据
	menus, permissions, permIDs, err := s.roleRepo.GetRolePermission(roleID)
	if err != nil {
		return nil, err
	}

	// Step 2: 构建菜单树（业务逻辑）
	menuMap := make(map[int]*dto.MenuTreeNodeRole)
	for _, m := range menus {
		node := &dto.MenuTreeNodeRole{
			ID:       m.ID,
			Title:    m.Title,
			Name:     "",
			Type:     "menu", // 或根据 m.Type 动态设为 "catalogue"
			Checked:  contains(permIDs, m.ID),
			Children: []*dto.MenuTreeNodeRole{},
		}
		menuMap[m.ID] = node
	}

	var roots []*dto.MenuTreeNodeRole
	for _, m := range menus {
		node := menuMap[m.ID]
		if m.ParentID == nil {
			roots = append(roots, node)
		} else if parentNode, exists := menuMap[*m.ParentID]; exists {
			parentNode.Children = append(parentNode.Children, node)
		}
	}

	// Step 3: 挂载权限节点（业务规则：权限通过 category 关联菜单）
	permSet := make(map[int]bool)
	for _, id := range permIDs {
		permSet[id] = true
	}

	for _, p := range permissions {
		parentID, ok := parseMenuIDFromCategory(p.Category)
		if !ok {
			continue
		}
		if parentNode, exists := menuMap[parentID]; exists {
			child := &dto.MenuTreeNodeRole{
				ID:       p.ID,
				Title:    p.Name,
				Name:     p.Name,
				Type:     "permission",
				Checked:  permSet[p.ID],
				Children: nil,
			}
			parentNode.Children = append(parentNode.Children, child)
		}
	}

	// Step 4: 构建 API 树（业务逻辑）
	apiTree := buildAPITree(permSet, permissions)

	return &dto.RolePermissionTree{
		Permissions: roots,
		API:         apiTree,
	}, nil
}

// 辅助函数
func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func parseMenuIDFromCategory(cat string) (int, bool) {
	if strings.HasPrefix(cat, "menu_") {
		idStr := strings.TrimPrefix(cat, "menu_")
		id, err := strconv.ParseUint(idStr, 10, 32)
		return int(id), err == nil
	}
	return 0, false
}

func buildAPITree(permSet map[int]bool, perms []entity.Permission) dto.APITree {
	apiTree := dto.APITree{
		Title:    "API权限",
		Checked:  false,
		Children: []*dto.APINode{},
	}

	grouped := make(map[string][]*dto.APINode)
	for _, p := range perms {
		if p.Type != "permission" {
			continue
		}
		node := &dto.APINode{
			HTTPPath: p.HTTPPath,
			Checked:  permSet[p.ID],
		}
		grouped[p.HTTPPath] = append(grouped[p.HTTPPath], node)
	}

	for path, nodes := range grouped {
		checked := true
		for _, n := range nodes {
			if !n.Checked {
				checked = false
				break
			}
		}
		apiTree.Children = append(apiTree.Children, &dto.APINode{
			HTTPPath: path,
			Checked:  checked,
		})
	}

	return apiTree
}

func (s *roleService) UpdateRolePermission(roleID int, permIDs []int) error {
	return s.roleRepo.UpdateRolePermission(roleID, permIDs)
}
