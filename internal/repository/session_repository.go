package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// 会话键前缀
	sessionKeyPrefix = "ssh:session:"

	// 会话列表键
	sessionListKey = "ssh:session:list"

	// 用户凭证键前缀
	credsKeyPrefix = "ssh:creds:"

	// 凭证默认过期时间（30分钟）
	defaultCredsTTL = 30 * time.Minute
)

// sessionRepository SSH会话仓库实现
type sessionRepository struct {
	redis *redis.Client
	ctx   context.Context
}

// NewSessionRepository 创建会话仓库
func NewSessionRepository(redisClient *redis.Client) SessionRepository {
	return &sessionRepository{
		redis: redisClient,
		ctx:   context.Background(),
	}
}

// getSessionKey 获取会话键
func (r *sessionRepository) getSessionKey(userID int) string {
	return fmt.Sprintf("%s%d", sessionKeyPrefix, userID)
}

// getCredsKey 获取凭证键
func (r *sessionRepository) getCredsKey(userID int) string {
	return fmt.Sprintf("%s%d", credsKeyPrefix, userID)
}

// SaveSession 保存会话元数据
func (r *sessionRepository) SaveSession(userID int, username string, isRoot bool, createdAt time.Time) error {
	if r.redis == nil {
		// Redis不可用时不报错，静默处理
		return nil
	}

	info := &SessionInfo{
		UserID:     userID,
		Username:   username,
		IsRoot:     isRoot,
		CreatedAt:  createdAt,
		LastUsedAt: createdAt,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("序列化会话信息失败: %w", err)
	}

	key := r.getSessionKey(userID)

	// 保存会话信息
	if err := r.redis.Set(r.ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("保存会话信息失败: %w", err)
	}

	// 添加到会话列表集合
	if err := r.redis.SAdd(r.ctx, sessionListKey, userID).Err(); err != nil {
		return fmt.Errorf("添加会话到列表失败: %w", err)
	}

	return nil
}

// GetSessionInfo 获取会话信息
func (r *sessionRepository) GetSessionInfo(userID int) (*SessionInfo, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis不可用")
	}

	key := r.getSessionKey(userID)
	data, err := r.redis.Get(r.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("会话不存在")
		}
		return nil, fmt.Errorf("获取会话信息失败: %w", err)
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("反序列化会话信息失败: %w", err)
	}

	return &info, nil
}

// DeleteSession 删除会话元数据
func (r *sessionRepository) DeleteSession(userID int) error {
	if r.redis == nil {
		return nil
	}

	key := r.getSessionKey(userID)

	// 删除会话信息
	if err := r.redis.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("删除会话信息失败: %w", err)
	}

	// 从会话列表中移除
	if err := r.redis.SRem(r.ctx, sessionListKey, userID).Err(); err != nil {
		return fmt.Errorf("从会话列表移除失败: %w", err)
	}

	return nil
}

// DeleteAllSessions 删除所有会话元数据
func (r *sessionRepository) DeleteAllSessions() error {
	if r.redis == nil {
		return nil
	}

	// 获取所有会话ID
	userIDs, err := r.redis.SMembers(r.ctx, sessionListKey).Result()
	if err != nil {
		return fmt.Errorf("获取会话列表失败: %w", err)
	}

	// 删除每个会话
	for _, userIDStr := range userIDs {
		key := fmt.Sprintf("%s%s", sessionKeyPrefix, userIDStr)
		if err := r.redis.Del(r.ctx, key).Err(); err != nil {
			continue
		}
	}

	// 删除会话列表
	if err := r.redis.Del(r.ctx, sessionListKey).Err(); err != nil {
		return fmt.Errorf("删除会话列表失败: %w", err)
	}

	return nil
}

// ListActiveSessions 列出所有活跃会话
func (r *sessionRepository) ListActiveSessions() ([]*SessionInfo, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("Redis不可用")
	}

	// 获取所有会话ID
	userIDs, err := r.redis.SMembers(r.ctx, sessionListKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取会话列表失败: %w", err)
	}

	sessions := make([]*SessionInfo, 0, len(userIDs))
	for _, userIDStr := range userIDs {
		key := fmt.Sprintf("%s%s", sessionKeyPrefix, userIDStr)
		data, err := r.redis.Get(r.ctx, key).Bytes()
		if err != nil {
			continue
		}

		var info SessionInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		sessions = append(sessions, &info)
	}

	return sessions, nil
}

// UpdateLastUsed 更新最后使用时间
func (r *sessionRepository) UpdateLastUsed(userID int, lastUsedAt time.Time) error {
	if r.redis == nil {
		return nil
	}

	key := r.getSessionKey(userID)
	data, err := r.redis.Get(r.ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("获取会话信息失败: %w", err)
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("反序列化会话信息失败: %w", err)
	}

	info.LastUsedAt = lastUsedAt

	newData, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("序列化会话信息失败: %w", err)
	}

	if err := r.redis.Set(r.ctx, key, newData, 0).Err(); err != nil {
		return fmt.Errorf("更新会话信息失败: %w", err)
	}

	return nil
}

// SaveUserCredentials 保存用户SSH凭证（用于终端重连）
func (r *sessionRepository) SaveUserCredentials(userID int, username, password string, expiresAt time.Time) error {
	if r.redis == nil {
		return nil
	}

	creds := &UserCredentials{
		UserID:    userID,
		Username:  username,
		Password:  password,
		ExpiresAt: expiresAt,
	}

	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("序列化凭证失败: %w", err)
	}

	key := r.getCredsKey(userID)

	// 计算TTL
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = defaultCredsTTL
	}

	// 保存凭证
	if err := r.redis.Set(r.ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("保存凭证失败: %w", err)
	}

	return nil
}

// GetUserCredentials 获取用户SSH凭证
func (r *sessionRepository) GetUserCredentials(userID int) (*UserCredentials, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("Redis不可用")
	}

	key := r.getCredsKey(userID)
	data, err := r.redis.Get(r.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("凭证不存在或已过期")
		}
		return nil, fmt.Errorf("获取凭证失败: %w", err)
	}

	var creds UserCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("反序列化凭证失败: %w", err)
	}

	// 检查凭证是否过期
	if time.Now().After(creds.ExpiresAt) {
		_ = r.DeleteUserCredentials(userID)
		return nil, fmt.Errorf("凭证已过期")
	}

	return &creds, nil
}

// DeleteUserCredentials 删除用户SSH凭证
func (r *sessionRepository) DeleteUserCredentials(userID int) error {
	if r.redis == nil {
		return nil
	}

	key := r.getCredsKey(userID)
	if err := r.redis.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("删除凭证失败: %w", err)
	}

	return nil
}
