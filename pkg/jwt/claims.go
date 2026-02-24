package jwt

import "github.com/golang-jwt/jwt/v5"

// CustomClaims 自定义 JWT Claims
type CustomClaims struct {
	UserID   int      `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// GetUserID 获取用户 ID
func (c *CustomClaims) GetUserID() int {
	return c.UserID
}

// GetUsername 获取用户名
func (c *CustomClaims) GetUsername() string {
	return c.Username
}

