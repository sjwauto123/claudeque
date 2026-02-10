package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/captcha"
	"cloudque/pkg/email"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/jwt"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// authService 认证服务实现
type authService struct {
	userRepo    repository.UserRepository
	roleRepo    repository.RoleRepository
	redisRepo   repository.RedisRepository
	userService UserService
}

// NewAuthService 创建认证服务
func NewAuthService(userRepo repository.UserRepository, roleRepo repository.RoleRepository, redisRepo repository.RedisRepository, userService UserService) AuthService {
	return &authService{
		userRepo:    userRepo,
		roleRepo:    roleRepo,
		redisRepo:   redisRepo,
		userService: userService,
	}
}

// Login 用户登录
func (s *authService) Login(req *request.LoginRequest) (*dto.LoginResponse, error) {
	// 校验验证码
	if !captcha.Verify(req.CaptchaID, req.Captcha) {
		return nil, bizerrors.ErrInvalidCaptcha
	}

	// 查找用户并校验存在性
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 检查用户状态是否启用
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 构建角色列表
	roles := s.buildRoles(user)
	// 聚合权限和菜单
	permsDTO, menus := s.aggregate(user)
	// 构建菜单树
	menuNodes := buildMenuTree(menus)

	// 生成 JWT
	token, err := jwt.GenerateToken(user.ID, user.Username, roles)
	if err != nil {
		return nil, err
	}

	// 组装用户信息
	userResp := s.userService.GetUserResponse(user)
	// 返回登录响应数据
	return &dto.LoginResponse{
		Token:       token,
		User:        *userResp,
		Permissions: permsDTO,
		MenusTree:   menuNodes,
	}, nil
}

// 聚合角色
func (s *authService) buildRoles(user *entity.User) []string {
	roles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		if r.Status == 1 {
			roles = append(roles, r.Slug)
		}
	}
	return roles
}

// 将权限和菜单聚合到map中，并以固定形式返回
func (s *authService) aggregate(user *entity.User) ([]dto.Permission, []entity.Menu) {
	permMap := make(map[int]entity.Permission)
	menuMap := make(map[int]entity.Menu)
	for _, r := range user.Roles {
		if r.Status != 1 {
			continue
		}
		role, err := s.roleRepo.FindBySlug(r.Slug)
		if err != nil || role == nil {
			continue
		}
		for _, p := range role.Permissions {
			if p.Status == 1 {
				permMap[p.ID] = p
			}
		}
		for _, m := range role.Menus {
			if m.Status == 1 {
				menuMap[m.ID] = m
			}
		}
	}
	permsDTO := make([]dto.Permission, 0, len(permMap))
	for _, p := range permMap {
		permsDTO = append(permsDTO, dto.Permission{
			ID:         p.ID,
			Name:       p.Name,
			Category:   p.Category,
			Slug:       p.Slug,
			Type:       p.Type,
			Status:     p.Status,
			HttpMethod: p.HttpMethod,
			HttpPath:   p.HttpPath,
			Sort:       p.Sort,
			CreatedAt:  p.CreatedAt,
			UpdatedAt:  p.UpdatedAt,
		})
	}
	menus := make([]entity.Menu, 0, len(menuMap))
	for _, m := range menuMap {
		menus = append(menus, m)
	}
	return permsDTO, menus
}

// 构建菜单树
func buildMenuTree(menus []entity.Menu) []dto.MenuNode {
	nodeMap := make(map[int]*dto.MenuNode)
	parentChildren := make(map[int][]dto.MenuNode)
	for _, m := range menus {
		nodeMap[m.ID] = &dto.MenuNode{
			ID:       m.ID,
			ParentID: m.ParentID,
			Title:    m.Title,
			Status:   m.Status,
			Type:     m.Type,
			Icon:     m.Icon,
			URI:      m.URI,
			Sort:     m.Sort,
		}
	}
	for _, n := range nodeMap {
		if n.ParentID != 0 {
			parentChildren[n.ParentID] = append(parentChildren[n.ParentID], *n)
		}
	}
	menuNodes := make([]dto.MenuNode, 0)
	for id, n := range nodeMap {
		if ch, ok := parentChildren[id]; ok {
			sort.Slice(ch, func(i, j int) bool {
				if ch[i].Sort == ch[j].Sort {
					return ch[i].Title < ch[j].Title
				}
				return ch[i].Sort < ch[j].Sort
			})
			n.Children = ch
		}
		if n.ParentID == 0 || nodeMap[n.ParentID] == nil {
			menuNodes = append(menuNodes, *n)
		}
	}
	sort.Slice(menuNodes, func(i, j int) bool {
		if menuNodes[i].Sort == menuNodes[j].Sort {
			return menuNodes[i].Title < menuNodes[j].Title
		}
		return menuNodes[i].Sort < menuNodes[j].Sort
	})
	return menuNodes
}

// RefreshToken 刷新 Token
func (s *authService) RefreshToken(token string) (string, error) {
	newToken, err := jwt.RefreshToken(token)
	if err != nil {
		return "", err
	}
	return newToken, nil
}

// SendEmailCode 发送邮箱验证码
func (s *authService) SendEmailCode(emailStr string) error {
	// 1. 生成6位随机数字
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := fmt.Sprintf("%06d", rnd.Intn(1000000))

	// 2. 存储到 Redis (有效期5分钟)
	ctx := context.Background()
	key := fmt.Sprintf("email_code:%s", emailStr)
	err := s.redisRepo.Set(ctx, key, code, 5*time.Minute)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "缓存验证码失败", err)
	}

	// 3. 发送邮件
	subject := "您的验证码"
	body := fmt.Sprintf("<h1>您的验证码是: %s</h1><p>有效期5分钟，请勿泄露给他人。</p>", code)
	if err := email.SendEmail(emailStr, subject, body); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "发送邮件失败", err)
	}

	return nil
}

// GetPermissionsByRole 根据角色 Slug 获取权限列表
func (s *authService) GetPermissionsByRole(slug string) ([]entity.Permission, error) {
	role, err := s.roleRepo.FindBySlug(slug)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "角色不存在")
	}

	return role.Permissions, nil
}
