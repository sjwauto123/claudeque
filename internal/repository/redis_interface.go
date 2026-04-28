package repository

import (
	"context"
	"time"
)

// RedisRepository Redis 仓储接口
type RedisRepository interface {
	// Set 设置缓存
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	// Get 获取缓存
	Get(ctx context.Context, key string) (string, error)
	// Del 删除缓存
	Del(ctx context.Context, key string) error

	// SAdd 向集合添加成员
	SAdd(ctx context.Context, key string, members ...interface{}) error
	// SRem 从集合移除成员
	SRem(ctx context.Context, key string, members ...interface{}) error
	// SMembers 获取集合所有成员
	SMembers(ctx context.Context, key string) ([]string, error)
	// SIsMember 判断成员是否在集合中
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)

	// HSet 设置哈希字段值
	HSet(ctx context.Context, key string, values map[string]interface{}) error
	// HGet 获取哈希字段值
	HGet(ctx context.Context, key string, field string) (string, error)
	// HGetAll 获取哈希所有字段和值
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	// HDel 删除哈希字段
	HDel(ctx context.Context, key string, fields ...string) error
}
