package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 用户登录
	Login(req *request.LoginRequest) (*dto.LoginResponse, error)
	// RefreshToken 刷新 Token
	RefreshToken(token string) (string, error)
	// SendEmailCode 发送邮箱验证码
	SendEmailCode(email string) error
	// GetPermissionsByRole 根据角色 Slug 获取权限列表
	GetPermissionsByRole(slug string) ([]entity.Permission, error)
	// GetAllRoles 获取所有角色
	GetAllRoles() ([]entity.Role, error)
}
