package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"cloudque/pkg/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// userService 用户服务实现
type userService struct {
	userRepo  repository.UserRepository
	redisRepo repository.RedisRepository
	sshConfig *ssh.Config
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository, redisRepo repository.RedisRepository, sshConfig *ssh.Config) UserService {
	return &userService{
		userRepo:  userRepo,
		redisRepo: redisRepo,
		sshConfig: sshConfig,
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

	pwd := utils.DecryptIfCryptoJS(req.Password)
	log.Println(pwd)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
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

	// 在VM中创建用户
	if s.sshConfig != nil && s.sshConfig.ServerHost != "" && s.sshConfig.PrivateKeyPath != "" {
		if err := s.createVMUser(req.Username, pwd); err != nil {
			logger.Error("VM用户创建失败", zap.String("username", req.Username), zap.Error(err))
			// 回滚数据库：删除已创建的用户
			if delErr := s.userRepo.Delete(user.ID); delErr != nil {
				logger.Error("回滚用户失败", zap.Int("id", user.ID), zap.Error(delErr))
			}
			return bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建虚拟机用户失败", err)
		}
		logger.Info("VM用户创建成功", zap.String("username", req.Username))
	}

	return nil
}

// createVMUser 在虚拟机中创建用户
func (s *userService) createVMUser(username, password string) error {
	config := &server.Config{
		Host:                 s.sshConfig.ServerHost,
		Username:             s.sshConfig.RootUsername,
		Password:             s.sshConfig.RootPassword,
		PrivateKeyPath:       s.sshConfig.PrivateKeyPath,
		PrivateKeyPassphrase: s.sshConfig.PrivateKeyPassphrase,
		Timeout:              s.sshConfig.Timeout,
	}

	client, err := server.NewClient(config)
	if err != nil {
		return fmt.Errorf("connect to vm failed: %w", err)
	}
	defer client.Close()

	// 1. Check if user exists
	checkCmd := fmt.Sprintf("id -u %s", username)
	if _, err := client.ExecuteCommand(checkCmd); err == nil {
		logger.Warn("user already exists in VM", zap.String("username", username))
		return nil
	}

	// 2. Create user
	createCmd := fmt.Sprintf("useradd -m -s /bin/bash %s", username)
	if out, err := client.ExecuteCommand(createCmd); err != nil {
		return fmt.Errorf("useradd failed: %s, error: %w", out, err)
	}

	// 3. Set password
	// Escape single quotes in password for shell safety
	safePassword := strings.ReplaceAll(password, "'", "'\\''")
	passCmd := fmt.Sprintf("echo '%s:%s' | chpasswd", username, safePassword)
	if out, err := client.ExecuteCommand(passCmd); err != nil {
		return fmt.Errorf("chpasswd failed: %s, error: %w", out, err)
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

func (s *userService) UpdateAvatar(id int, avatarPath string) error {
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
func (s *userService) ChangePassword(id int, req *request.ChangePasswordRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	oldPwd := utils.DecryptIfCryptoJS(req.OldPassword)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 加密新密码
	newPwd := utils.DecryptIfCryptoJS(req.NewPassword)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 1. 同步修改虚拟机密码 (方案A：先修改VM，失败则终止)
	if s.sshConfig != nil && s.sshConfig.ServerHost != "" && s.sshConfig.PrivateKeyPath != "" {
		if err := s.updateVMPassword(user.Username, newPwd); err != nil {
			logger.Error("Failed to update VM password during ChangePassword", zap.String("username", user.Username), zap.Error(err))
			return bizerrors.NewWithErr(bizerrors.CodeInternalError, "同步虚拟机密码失败，请稍后重试", err)
		}
	}

	// 2. 修改数据库密码
	user.Password = string(hashedPassword)
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	return nil
}

// updateVMPassword 修改虚拟机中的用户密码
func (s *userService) updateVMPassword(username, password string) error {
	config := &server.Config{
		Host:                 s.sshConfig.ServerHost,
		Username:             s.sshConfig.RootUsername,
		Password:             s.sshConfig.RootPassword,
		PrivateKeyPath:       s.sshConfig.PrivateKeyPath,
		PrivateKeyPassphrase: s.sshConfig.PrivateKeyPassphrase,
		Timeout:              s.sshConfig.Timeout,
	}

	client, err := server.NewClient(config)
	if err != nil {
		return fmt.Errorf("connect to vm failed: %w", err)
	}
	defer client.Close()

	// Set password
	// Escape single quotes in password for shell safety
	safePassword := strings.ReplaceAll(password, "'", "'\\''")
	passCmd := fmt.Sprintf("echo '%s:%s' | chpasswd", username, safePassword)
	if out, err := client.ExecuteCommand(passCmd); err != nil {
		return fmt.Errorf("chpasswd failed: %s, error: %w", out, err)
	}

	return nil
}

// deleteVMUser 删除虚拟机中的用户
func (s *userService) deleteVMUser(username string) error {
	config := &server.Config{
		Host:                 s.sshConfig.ServerHost,
		Username:             s.sshConfig.RootUsername,
		Password:             s.sshConfig.RootPassword,
		PrivateKeyPath:       s.sshConfig.PrivateKeyPath,
		PrivateKeyPassphrase: s.sshConfig.PrivateKeyPassphrase,
		Timeout:              s.sshConfig.Timeout,
	}

	client, err := server.NewClient(config)
	if err != nil {
		return fmt.Errorf("connect to vm failed: %w", err)
	}
	defer client.Close()

	// Check if user exists
	checkCmd := fmt.Sprintf("id -u %s", username)
	if _, err := client.ExecuteCommand(checkCmd); err != nil {
		// User not found, consider as success
		return nil
	}

	// Delete user
	// -r: remove home directory and mail spool
	deleteCmd := fmt.Sprintf("userdel -r %s", username)
	if out, err := client.ExecuteCommand(deleteCmd); err != nil {
		return fmt.Errorf("userdel failed: %s, error: %w", out, err)
	}

	return nil
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

	newPwd := utils.DecryptIfCryptoJS(req.NewPassword)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 3. 同步修改虚拟机密码
	if s.sshConfig != nil && s.sshConfig.ServerHost != "" && s.sshConfig.PrivateKeyPath != "" {
		if err := s.updateVMPassword(user.Username, newPwd); err != nil {
			logger.Error("Failed to update VM password during ResetPassword", zap.String("username", user.Username), zap.Error(err))
			return bizerrors.NewWithErr(bizerrors.CodeInternalError, "同步虚拟机密码失败，请稍后重试", err)
		}
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
	users, total, err := s.userRepo.List(offset, size, req.Username, req.Email, req.Status)
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
