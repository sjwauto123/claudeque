package service

// 回归测试：显卡可用性判定。
//
// 背景（线上故障）：gpu:status:* 缓存 key 因 24 小时 TTL 到期而全部消失后，
// 缓存把「未命中」当成「不可用」返回 (false, nil)，service 层据此短路，
// 导致任务永久停在 status=6，显卡空闲却永远排不上。
//
// 判定语义必须是这三种：
//   1. 缓存未命中       -> 状态「未知」 -> 必须回退查数据库
//   2. 缓存命中且忙     -> 状态「确定不可用」 -> 短路，不必查数据库
//   3. 缓存命中且空闲   -> 状态「可能可用」 -> 仍回查数据库确认
//
// 本测试直接构造 MySQL 与 Redis 客户端，不依赖 logger / 全局 config 的初始化。
// 连接信息全部从环境变量读取，连不上则自动跳过，因此不会让 CI 变红：
//
//	TEST_REDIS_ADDR    默认 127.0.0.1:6379
//	TEST_REDIS_DB      默认 1
//	TEST_REDIS_PASSWORD 默认空
//	TEST_MYSQL_DSN     无默认值，未设置则跳过；形如
//	                   user:pass@tcp(127.0.0.1:3306)/cloudque?charset=utf8mb4&parseTime=True&loc=Local

import (
	"context"
	"os"
	"strconv"
	"testing"

	"cloudque/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	cacheKeyPrefix = "gpu:status:"
	// realCardID 是数据库 gpu_cards 里真实存在、且 status=0（空闲）的显卡，即 gpu-0
	realCardID = 3035
	// fakeCardID 在数据库里不存在，用来区分「缓存短路」与「数据库判定」两条路径
	fakeCardID = 999999
)

func newTestGpuService(t *testing.T) (GpuService, *redis.Client) {
	t.Helper()
	ctx := context.Background()

	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	redisDB := 1
	if v := os.Getenv("TEST_REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			redisDB = n
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("TEST_REDIS_PASSWORD"),
		DB:       redisDB,
		PoolSize: 10,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("Redis 不可用，跳过本测试: %v", err)
	}

	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		_ = rdb.Close()
		t.Skip("未设置 TEST_MYSQL_DSN，跳过本测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		_ = rdb.Close()
		t.Skipf("MySQL 不可用，跳过本测试: %v", err)
	}

	gpuRepo := repository.NewGpuRepository(db)
	gpuCache := repository.NewGpuCacheRepository(rdb)
	return NewGpuService(gpuRepo, gpuCache), rdb
}

// 用例 1（核心回归）：缓存未命中，必须回退数据库，返回可用
func TestCheckAvailable_CacheMissFallsBackToDatabase(t *testing.T) {
	svc, rdb := newTestGpuService(t)
	defer rdb.Close()
	ctx := context.Background()

	// 复现线上故障现场：缓存里没有这张卡
	if err := rdb.Del(ctx, cacheKeyPrefix+"3035").Err(); err != nil {
		t.Fatalf("清理缓存 key 失败: %v", err)
	}
	if n := rdb.Exists(ctx, cacheKeyPrefix+"3035").Val(); n != 0 {
		t.Fatalf("前置条件不成立: gpu:status:3035 仍然存在")
	}

	got, err := svc.CheckAvailable(ctx, []int{realCardID})
	if err != nil {
		t.Fatalf("CheckAvailable 返回了错误: %v", err)
	}
	if !got {
		t.Fatalf("回归失败: 缓存未命中时没有回退数据库，返回了 false；" +
			"而数据库里这张卡是空闲的，说明 fail-closed 的逻辑仍然存在")
	}
	t.Logf("通过: 缓存未命中 -> 回退数据库 -> available=%v", got)
}

// 用例 2：缓存中明确记录为「忙」，应当短路，不必查数据库
func TestCheckAvailable_CacheHitBusyShortCircuits(t *testing.T) {
	svc, rdb := newTestGpuService(t)
	defer rdb.Close()
	ctx := context.Background()

	key := cacheKeyPrefix + "999999"
	if err := rdb.HSet(ctx, key, "status", 1, "job_id", 12345).Err(); err != nil {
		t.Fatalf("写入缓存失败: %v", err)
	}
	defer func() { _ = rdb.Del(ctx, key).Err() }()

	got, err := svc.CheckAvailable(ctx, []int{fakeCardID})
	if err != nil {
		t.Fatalf("CheckAvailable 返回了错误: %v", err)
	}
	if got {
		t.Fatalf("缓存明确记录为忙，却返回了可用")
	}
	t.Logf("通过: 缓存命中且忙 -> 直接短路 -> available=%v", got)
}

// 用例 3：缓存命中且空闲，仍应回查数据库确认（数据库没有这张卡，所以应为不可用）
func TestCheckAvailable_CacheHitIdleStillVerifiesWithDatabase(t *testing.T) {
	svc, rdb := newTestGpuService(t)
	defer rdb.Close()
	ctx := context.Background()

	key := cacheKeyPrefix + "999999"
	if err := rdb.HSet(ctx, key, "status", 0, "job_id", "").Err(); err != nil {
		t.Fatalf("写入缓存失败: %v", err)
	}
	defer func() { _ = rdb.Del(ctx, key).Err() }()

	got, err := svc.CheckAvailable(ctx, []int{fakeCardID})
	if err != nil {
		t.Fatalf("CheckAvailable 返回了错误: %v", err)
	}
	if got {
		t.Fatalf("数据库中不存在这张卡，却返回了可用，说明跳过了数据库校验")
	}
	t.Logf("通过: 缓存命中且空闲 -> 仍回查数据库 -> available=%v", got)
}

// 用例 4：直接验证缓存仓储层，未命中必须返回非 nil 错误
func TestGpuCacheCheckAvailable_CacheMissReturnsError(t *testing.T) {
	_, rdb := newTestGpuService(t)
	defer rdb.Close()
	ctx := context.Background()
	cache := repository.NewGpuCacheRepository(rdb)

	key := cacheKeyPrefix + "999999"
	if err := rdb.Del(ctx, key).Err(); err != nil {
		t.Fatalf("清理缓存 key 失败: %v", err)
	}

	avail, err := cache.CheckAvailable(ctx, []int{fakeCardID})
	if err == nil {
		t.Fatalf("缓存未命中却返回了 nil 错误，上层将无法回退数据库")
	}
	if avail {
		t.Fatalf("缓存未命中时不应返回 available=true")
	}
	t.Logf("通过: 缓存未命中 -> available=%v, err=%v", avail, err)
}
