package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
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
	redisRepo   repository.RedisRepository
	userService UserService
}

// NewAuthService 创建认证服务
func NewAuthService(userRepo repository.UserRepository, redisRepo repository.RedisRepository, userService UserService) AuthService {
	return &authService{
		userRepo:    userRepo,
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

	// 获取用户角色（模拟）
	// TODO: 实现 getUserRoles(user.ID)
	// roles, err := s.roleRepo.GetRolesByUserID(user.ID)
	// 目前暂时硬编码为 admin 角色以便测试，或者空
	roles := []string{"admin"} // 示例角色

	// 生成 Token
	token, err := jwt.GenerateToken(user.ID, user.Username, roles)
	if err != nil {
		return nil, err
	}

	// 构建响应
	userResp := s.userService.GetUserResponse(user)
	return &dto.LoginResponse{
		Token: token,
		User:  *userResp,
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
