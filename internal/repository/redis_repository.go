package repository

import (
	"cloudque/pkg/database"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisRepository struct {
	client *redis.Client
}

// NewRedisRepository 创建 Redis 仓储
func NewRedisRepository() RedisRepository {
	return &redisRepository{
		client: database.GetRedis(),
	}
}

// Set 设置缓存
func (r *redisRepository) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取缓存
func (r *redisRepository) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Del 删除缓存
func (r *redisRepository) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
