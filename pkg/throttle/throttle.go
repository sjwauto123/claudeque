package throttle

import (
	"sync"
	"time"

	"cloudque/pkg/logger"
)

// ErrorThrottle 错误日志限流器
type ErrorThrottle struct {
	mu          sync.Mutex
	lastLogTime time.Time
	interval    time.Duration
}

// NewErrorThrottle 创建限流器（interval 是最小打印间隔）
func NewErrorThrottle(interval time.Duration) *ErrorThrottle {
	return &ErrorThrottle{interval: interval}
}

// Log 打印错误日志（自动限流）
func (t *ErrorThrottle) Log(msg string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if now.Sub(t.lastLogTime) >= t.interval {
		logger.Errorf(msg, args...)
		t.lastLogTime = now
	}
}
