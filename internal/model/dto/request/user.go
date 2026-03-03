package request

import (
	"cloudque/pkg/response"
)

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	Username        string `json:"username" binding:"required,min=3,max=50"`
	Password        string `json:"password" binding:"required,min=6,max=300"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	EmailCaptcha    string `json:"captcha" binding:"required,len=6"`
}

// LoginRequest 用户登录请求
type LoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	CaptchaID string `json:"captcha_id" binding:"required"`
	Captcha   string `json:"captcha" binding:"required,len=4"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6,max=50"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// UserListRequest 用户列表请求
type UserListRequest struct {
	response.PageRequest
	Username string `form:"username" json:"username"`
	Email    string `form:"email" json:"email"`
	Status   string `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
}

type GetByUsernameRequest struct {
	Username string `json:"username" binding:"required"`
}

// CreateRequest 新增用户请求
type CreateRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
	Email    string `json:"email" binding:"required,email"`

	Status int       `json:"status"`
	Roles  *[]string `json:"roles" binding:"omitempty"`
}

// AdminUpdateUserRequest 管理员更新用户信息请求
type AdminUpdateUserRequest struct {
	Email         string    `json:"email" binding:"omitempty,email"`
	Status        *int      `json:"status" binding:"omitempty,oneof=0 1"` // 0:禁用 1:正常
	Password      string    `json:"password" binding:"omitempty,min=6,max=50"`
	Priority      *int      `json:"priority" binding:"omitempty,oneof=1 2"`
	MultiTraining *int      `json:"multi_training" binding:"omitempty,oneof=0 1"`
	CrossServer   *int      `json:"cross_server" binding:"omitempty,oneof=0 1"`
	Roles         *[]string `json:"roles" binding:"omitempty"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Email           string `json:"email" binding:"required,email"`
	EmailCaptcha    string `json:"captcha" binding:"required,len=6"`
	NewPassword     string `json:"password" binding:"required,min=6,max=50"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}
