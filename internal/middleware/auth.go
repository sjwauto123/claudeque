package middleware

import (
	"strings"

	"cloudque/internal/service"
	"cloudque/pkg/jwt"
	"cloudque/pkg/response"

	"github.com/gin-gonic/gin"
)

const (
	// ContextUserID 用户 ID 上下文键
	ContextUserID = "user_id"
	// ContextUsername 用户名 上下文键
	ContextUsername = "username"
	// ContextRoles 角色 上下文键
	ContextRoles = "roles"
)

// Auth JWT 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "请提供认证令牌")
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "令牌格式错误")
			c.Abort()
			return
		}

		// 解析 Token
		claims, err := jwt.ParseToken(parts[1])
		if err != nil {
			response.Unauthorized(c, err.Error())
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(ContextUserID, claims.GetUserID())
		c.Set(ContextUsername, claims.GetUsername())
		c.Set(ContextRoles, claims.GetRoles())

		c.Next()
	}
}

// RequirePermission 权限检查中间件
func RequirePermission(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {

		// 取出 userID
		userIDInterface, exists := c.Get(ContextUserID)
		if !exists {
			response.Forbidden(c, "未登录")
			c.Abort()
			return
		}

		userID, ok := userIDInterface.(int)
		if !ok {
			response.Forbidden(c, "用户信息异常")
			c.Abort()
			return
		}

		// 获取当前请求信息
		method := c.Request.Method
		path := c.FullPath()

		// 数据库判断
		hasPermission, err := authService.CheckUserPermission(userID, method, path)
		if err != nil {
			response.Forbidden(c, "权限校验失败")
			c.Abort()
			return
		}

		if !hasPermission {
			response.Forbidden(c, "权限不足")
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) int {
	if userID, exists := c.Get(ContextUserID); exists {
		return userID.(int)
	}
	return 0
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(ContextUsername); exists {
		return username.(string)
	}
	return ""
}

// OptionalAuth 可选的 JWT 认证中间件
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := jwt.ParseToken(parts[1])
		if err == nil {
			c.Set(ContextUserID, claims.GetUserID())
			c.Set(ContextUsername, claims.GetUsername())
			c.Set(ContextRoles, claims.GetRoles())
		}

		c.Next()
	}
}
