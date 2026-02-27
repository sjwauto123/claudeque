package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"
)

// SystemAccessType 系统权限类型
type SystemAccessType string

const (
	AccessTypeTerminal SystemAccessType = "terminal"
	AccessTypeFile     SystemAccessType = "file"
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
	// CheckUserPermission 检查用户是否拥有权限
	CheckUserPermission(userID int, method string, path string) (bool, error)
	// EnsureSSHSession 确保用户的SSH会话存在 (默认身份)
	EnsureSSHSession(userID int) error
	// EnsureSSHSessionByType 确保特定身份的SSH会话存在
	EnsureSSHSessionByType(userID int, isRoot bool) error
	// SetSSHServerHost 设置SSH服务器地址
	SetSSHServerHost(host string)
	// SetSSHTimeout 设置SSH连接超时
	SetSSHTimeout(timeout time.Duration)
	// HasSystemAccess 检查用户是否拥有系统级权限 (根据访问类型)
	HasSystemAccess(userID int, accessType SystemAccessType) (bool, error)
}
