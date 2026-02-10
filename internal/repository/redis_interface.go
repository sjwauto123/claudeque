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
}
