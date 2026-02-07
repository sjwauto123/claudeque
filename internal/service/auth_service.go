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
	// 验证验证码
	if !captcha.Verify(req.CaptchaID, req.Captcha) {
		return nil, bizerrors.ErrInvalidCaptcha
	}

	// 查找用户
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	roles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		if r.Status == 1 {
			roles = append(roles, r.Slug)
		}
	}

	// 聚合用户权限（按角色去重，返回完整权限对象）
	permMap := make(map[uint]entity.Permission)
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
	}
	permissions := make([]entity.Permission, 0, len(permMap))
	for _, p := range permMap {
		permissions = append(permissions, p)
	}

	// 生成 Token
	token, err := jwt.GenerateToken(user.ID, user.Username, roles)
	if err != nil {
		return nil, err
	}

	// 构建响应
	userResp := s.userService.GetUserResponse(user)
	return &dto.LoginResponse{
		Token:       token,
		User:        *userResp,
		Permissions: permissions,
	}, nil
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
