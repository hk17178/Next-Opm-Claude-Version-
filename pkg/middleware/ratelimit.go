// ratelimit.go 实现基于令牌桶算法的按 IP 限流中间件，防止单个客户端过度消耗服务资源。

package middleware

import (
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RateLimiterConfig 配置令牌桶限流器。
type RateLimiterConfig struct {
	Rate            float64       // 每秒每个客户端 IP 允许的请求数
	Burst           int           // 最大突发请求数（令牌桶容量）
	CleanupInterval time.Duration // 过期条目清理间隔
}

// tokenBucket 表示单个客户端的令牌桶状态
type tokenBucket struct {
	tokens    float64   // 当前可用令牌数
	lastCheck time.Time // 上次检查时间，用于计算令牌补充
}

// RateLimiter 实现基于令牌桶算法的按 IP 限流器。
type RateLimiter struct {
	rate    float64
	burst   int
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	logger  *zap.Logger
}

// NewRateLimiter 创建一个新的限流器实例，并启动后台过期条目清理协程。
func NewRateLimiter(cfg RateLimiterConfig, logger *zap.Logger) *RateLimiter {
	if cfg.Rate <= 0 {
		cfg.Rate = 100 // default 100 req/s
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 200
	}
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval == 0 {
		cleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		rate:    cfg.Rate,
		burst:   cfg.Burst,
		buckets: make(map[string]*tokenBucket),
		logger:  logger,
	}

	// Background cleanup of stale buckets
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

// allow 判断指定客户端是否允许通过，基于令牌桶算法按时间补充令牌
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &tokenBucket{
			tokens:    float64(rl.burst) - 1,
			lastCheck: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanup 清理超过 10 分钟未活跃的令牌桶条目，释放内存
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for key, b := range rl.buckets {
		if b.lastCheck.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}

// Middleware 返回按客户端 IP 执行限流的 HTTP 中间件。
// 超过限制时返回 429 状态码和 Retry-After 头。
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}

		if !rl.allow(clientIP) {
			rl.logger.Warn("rate limit exceeded",
				zap.String("client_ip", clientIP),
				zap.String("path", r.URL.Path),
			)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"code":"rate_limit.exceeded","message":"too many requests"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
