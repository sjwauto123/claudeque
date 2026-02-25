package repository

import (
	"time"
)

// SessionRepository SSH会话仓库接口
// 注意：实际的SSH Client对象由SSH SessionManager管理
// 这里主要用于存储会话元数据，支持跨服务重启后的状态查询
type SessionRepository interface {
	// SaveSession 保存会话元数据
	SaveSession(userID int, username string, isRoot bool, createdAt time.Time) error

	// GetSessionInfo 获取会话信息
	GetSessionInfo(userID int) (*SessionInfo, error)

	// DeleteSession 删除会话元数据
	DeleteSession(userID int) error

	// DeleteAllSessions 删除所有会话元数据
	DeleteAllSessions() error

	// ListActiveSessions 列出所有活跃会话
	ListActiveSessions() ([]*SessionInfo, error)

	// UpdateLastUsed 更新最后使用时间
	UpdateLastUsed(userID int, lastUsedAt time.Time) error

	// SaveUserCredentials 保存用户SSH凭证（用于终端重连）
	SaveUserCredentials(userID int, username, password string, expiresAt time.Time) error

	// GetUserCredentials 获取用户SSH凭证
	GetUserCredentials(userID int) (*UserCredentials, error)

	// DeleteUserCredentials 删除用户SSH凭证
	DeleteUserCredentials(userID int) error
}

// SessionInfo 会话信息
type SessionInfo struct {
	UserID     int       `json:"user_id"`
	Username   string    `json:"username"`
	IsRoot     bool      `json:"is_root"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

// UserCredentials 用户SSH凭证（用于终端重连）
type UserCredentials struct {
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	ExpiresAt time.Time `json:"expires_at"`
}
