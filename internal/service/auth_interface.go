package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"
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
	// EnsureSSHSession 确保用户的SSH会话存在 (默认身份)
	EnsureSSHSession(userID int) error
	// EnsureSSHSessionByType 确保特定身份的SSH会话存在
	EnsureSSHSessionByType(userID int, isRoot bool) error
	// SetSSHServerHost 设置SSH服务器地址
	SetSSHServerHost(host string)
	// SetSSHTimeout 设置SSH连接超时
	SetSSHTimeout(timeout time.Duration)
}
