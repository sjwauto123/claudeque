package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	bizerrors "cloudque/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// userService 用户服务实现
type userService struct {
	userRepo repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
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
		Nickname: req.Nickname,
		Status:   1, // 默认正常
	}

	if err := s.userRepo.Create(user); err != nil {
		return err
	}

	return nil
}

// GetUserByID 根据 ID 获取用户
func (s *userService) GetUserByID(id int) (*entity.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(id int, req *request.UpdateUserRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 更新字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	return s.userRepo.Update(user)
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(id int, req *request.ChangePasswordRequest) error {
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

// GetUserResponse 获取用户响应
func (s *userService) GetUserResponse(user *entity.User) *response.UserResponse {
	return &response.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
