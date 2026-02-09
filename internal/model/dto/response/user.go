package response

import (
	"cloudque/internal/model/entity"
	"time"
)

// UserResponse 用户响应
type UserResponse struct {
	ID            int       `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	Avatar        string    `json:"avatar"`
	Status        int       `json:"status"`
	Priority      int       `json:"priority"`
	MultiTraining int       `json:"multi_training"`
	CrossServer   int       `json:"cross_server"`
	Roles         []string  `json:"roles"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token       string              `json:"token"`
	User        UserResponse        `json:"user"`
	Permissions []entity.Permission `json:"permissions"`
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}
