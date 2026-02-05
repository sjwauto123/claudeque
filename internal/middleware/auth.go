package middleware

import (
	"strings"

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
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取用户角色
		rolesInterface, exists := c.Get(ContextRoles)
		if !exists {
			response.Forbidden(c, "无权访问")
			c.Abort()
			return
		}
		roles := rolesInterface.([]string)

		// 2. 查到用户所有的权限
		// 注意：此处应调用 Service 或 Repository 层方法查询数据库
		// 例如: permissions := permissionService.GetPermissionsByRoles(roles)
		userPermissions := getPermissionsByRoles(roles)

		// 3. 找是否有传入的这个权限名
		hasPermission := false
		for _, p := range userPermissions {
			if p == permission {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			response.Forbidden(c, "权限不足")
			c.Abort()
			return
		}

		c.Next()
	}
}

// getPermissionsByRoles 根据角色获取权限列表 (模拟实现)
func getPermissionsByRoles(roles []string) []string {
	// 实际项目中应查询数据库或缓存
	// return permissionRepo.GetByRoles(roles)

	permissions := make([]string, 0)
	for _, role := range roles {
		switch role {
		case "admin":
			// admin 拥有所有权限
			permissions = append(permissions, "user_manage", "post_manage", "system_manage")
		case "user_manager":
			permissions = append(permissions, "user_manage")
		case "editor":
			permissions = append(permissions, "post_manage")
		}
	}
	return permissions
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) uint {
	if userID, exists := c.Get(ContextUserID); exists {
		return userID.(uint)
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
