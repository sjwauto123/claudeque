package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"time"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 用户登录
	Login(req *request.LoginRequest) (*dto.LoginResponse, error)
	// Logout 用户登出
	Logout(userID uint) error
	// RefreshToken 刷新 Token
	RefreshToken(token string) (string, error)
	// EnsureSSHSession 确保用户的SSH会话存在
	EnsureSSHSession(userID uint) error
	// SetSSHServerHost 设置SSH服务器地址
	SetSSHServerHost(host string)
	// SetSSHTimeout 设置SSH连接超时
	SetSSHTimeout(timeout time.Duration)
}
