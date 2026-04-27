package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/pkg/response"

	"cloudque/internal/model/entity"
)

// UserService 用户服务接口
type UserService interface {
	AdminService // 嵌入管理员服务接口
	// Register 用户注册
	Register(req *request.RegisterRequest) error
	// GetUserByID 根据 ID 获取用户
	GetUserByID(id int) (*entity.User, error)
	// GetUserByUsername 根据用户名获取用户
	GetUserByUsername(username string) (*entity.User, error)
	// ChangePassword 修改密码
	ChangePassword(id int, req *request.ChangePasswordRequest) error
	// ResetPassword 重置密码
	ResetPassword(req *request.ResetPasswordRequest) error
	// UpdateAvatar 更新头像路径
	UpdateAvatar(id int, avatarPath string) error
	// UpdateProfile 更新个人信息
	UpdateProfile(id int, req *request.UpdateProfileRequest) error
	// GetUserResponse 获取用户响应
	GetUserResponse(user *entity.User) *dto.UserResponse
	// ListUsers 分页获取用户列表
	ListUsers(req *request.UserListRequest) (*response.PageResponse, error)
}
