package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// userService 用户服务实现
type userService struct {
	userRepo  repository.UserRepository
	redisRepo repository.RedisRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository, redisRepo repository.RedisRepository) UserService {
	return &userService{
		userRepo:  userRepo,
		redisRepo: redisRepo,
	}
}

// Register 用户注册
func (s *userService) Register(req *request.RegisterRequest) error {
	// 检查用户名是否存在
	exists, err := s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.ErrUserAlreadyExists
	}

	// 检查邮箱是否存在
	exists, err = s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
	}

	//验证邮箱验证码
	ctx := context.Background()
	codeKey := fmt.Sprintf("email_code:%s", req.Email)
	cacheCode, err := s.redisRepo.Get(ctx, codeKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return bizerrors.New(bizerrors.CodeInvalidParam, "验证码已过期或未发送，请重新获取")
		}
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "获取验证码缓存失败", err)
	}

	if req.EmailCaptcha != cacheCode {
		return bizerrors.New(bizerrors.CodeInvalidParam, "验证码输入错误，请重新核对")
	}
	if delErr := s.redisRepo.Del(ctx, codeKey); delErr != nil {
		logger.Info("删除验证码失败")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 创建用户
	user := &entity.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   1, // 默认正常
	}

	if err := s.userRepo.Create(user); err != nil {
		return err
	}
	// 为注册用户赋默认角色：普通用户
	if err := s.userRepo.AssignRoleByName(user.ID, "user"); err != nil {
		return err
	}
	return nil
}

// GetUserByID 根据 ID 获取用户
func (s *userService) GetUserByID(id uint) (*entity.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

func (s *userService) GetUserByUsername(username string) (*entity.User, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(id uint, req *request.UpdateUserRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	user.Username = req.Username

	return s.userRepo.Update(user)
}

func (s *userService) UpdateAvatar(id uint, avatarPath string) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 删除旧头像
	if user.Avatar != "" && user.Avatar != avatarPath {
		if err := os.Remove(user.Avatar); err != nil {
			// 如果文件不存在，忽略错误；其他错误记录日志
			if !os.IsNotExist(err) {
				logger.Error(fmt.Sprintf("Failed to remove old avatar: %s, error: %v", user.Avatar, err))
			}
		}
	}

	user.Avatar = avatarPath
	return s.userRepo.Update(user)
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(id uint, req *request.ChangePasswordRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return s.userRepo.Update(user)
}

// ResetPassword 重置密码
func (s *userService) ResetPassword(req *request.ResetPasswordRequest) error {
	// 1. 验证邮箱验证码
	ctx := context.Background()
	codeKey := fmt.Sprintf("email_code:%s", req.Email)
	cacheCode, err := s.redisRepo.Get(ctx, codeKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return bizerrors.New(bizerrors.CodeInvalidParam, "验证码已过期或未发送，请重新获取")
		}
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "获取验证码缓存失败", err)
	}

	if req.EmailCaptcha != cacheCode {
		return bizerrors.New(bizerrors.CodeInvalidParam, "验证码输入错误，请重新核对")
	}

	// 2. 查找用户
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return err
	}
	if user == nil {
		return bizerrors.New(bizerrors.CodeUserNotFound, "该邮箱未注册用户")
	}

	// 3. 更新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)

	// 4. 保存并删除验证码
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	_ = s.redisRepo.Del(ctx, codeKey)

	return nil
}

// GetUserResponse 获取用户响应
func (s *userService) GetUserResponse(user *entity.User) *dto.UserResponse {

	//roles := make([]string, 0, len(user.Roles))
	//for _, r := range user.Roles {
	//	roles = append(roles, r.Slug)
	//}

	roles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, r.Name)
	}

	return &dto.UserResponse{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Avatar:        user.Avatar,
		Status:        user.Status,
		Priority:      user.Priority,
		MultiTraining: user.MultiTraining,
		CrossServer:   user.CrossServer,
		Roles:         roles,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

// ListUsers 分页获取用户列表
func (s *userService) ListUsers(req *request.UserListRequest) (*response.PageResponse, error) {
	// 参数标准化
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	// 计算偏移量
	offset := (page - 1) * size

	// 查询数据
	users, total, err := s.userRepo.List(offset, size)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	list := make([]*dto.UserResponse, 0, len(users))
	for _, user := range users {
		list = append(list, s.GetUserResponse(user))
	}

	return response.NewPageResponse(list, total, page, size), nil
}
