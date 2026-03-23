package biz

import (
	"sync"
	"time"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

// CircuitState 表示熔断器的当前状态。
// 熔断器实现三态状态机：Closed（正常）→ Open（熔断）→ HalfOpen（试探恢复）。
type CircuitState int

// 熔断器状态常量。
//
// 状态流转规则：
//   - Closed → Open：连续失败次数达到 failureThreshold 时触发
//   - Open → HalfOpen：超过 timeout 时间后自动转入，允许放行一个试探请求
//   - HalfOpen → Closed：连续成功次数达到 successThreshold 时恢复
//   - HalfOpen → Open：任何一次失败立即重新熔断
const (
	StateClosed   CircuitState = iota // 正常运行，所有请求放行
	StateOpen                         // 熔断状态，拒绝所有请求以保护下游
	StateHalfOpen                     // 半开状态，允许少量试探请求检测服务是否恢复
)

// String 返回熔断器状态的字符串表示。
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreaker 为 AI 模型调用实现熔断器模式，防止持续向故障模型发送请求。
//
// 工作原理：
//   - 当连续失败次数超过阈值（failureThreshold），熔断器打开，拒绝后续所有调用
//   - 经过冷却时间（timeout）后，进入半开状态，放行一个试探请求
//   - 如果试探请求成功且连续成功达到阈值（successThreshold），恢复为关闭状态
//   - 如果试探请求失败，立即重新进入打开状态
//
// 线程安全：所有状态变更通过 sync.RWMutex 保护。
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailureTime  time.Time
	logger           *zap.Logger
}

// NewCircuitBreaker 根据配置创建熔断器实例，初始状态为 Closed（正常放行）。
func NewCircuitBreaker(cfg config.CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          time.Duration(cfg.TimeoutSeconds) * time.Second,
		logger:           logger,
	}
}

// Allow 检查当前是否允许请求通过熔断器。
// 在 Open 状态下，如果超过冷却时间则自动转入 HalfOpen 并放行。
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.logger.Info("circuit breaker transitioning to half-open")
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess 记录一次成功的调用。
// 在 HalfOpen 状态下，连续成功达到阈值后恢复为 Closed；
// 在 Closed 状态下，重置失败计数器。
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.logger.Info("circuit breaker closed (recovered)")
		}
	case StateClosed:
		cb.failureCount = 0
	}
}

// RecordFailure 记录一次失败的调用。
// 在 Closed 状态下，累计失败达到阈值则触发熔断（Open）；
// 在 HalfOpen 状态下，任何失败立即重新熔断。
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = StateOpen
			cb.logger.Warn("circuit breaker opened",
				zap.Int("failure_count", cb.failureCount),
				zap.Int("threshold", cb.failureThreshold),
			)
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.logger.Warn("circuit breaker re-opened from half-open state")
	}
}

// State 返回熔断器当前状态（只读，不触发状态转换）。
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen 返回熔断器是否处于打开状态（阻止请求）。
// 注意：如果已超过冷却时间，返回 false（表示下次 Allow 调用将转入 HalfOpen）。
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.timeout {
		return false // Will transition to half-open on next Allow()
	}
	return cb.state == StateOpen
}

// CircuitBreakerSnapshot 保存熔断器状态的快照，用于持久化到数据库和服务重启后恢复。
type CircuitBreakerSnapshot struct {
	State           string    `json:"state"`             // 熔断器状态：closed / open / half_open
	FailureCount    int       `json:"failure_count"`     // 当前连续失败计数
	SuccessCount    int       `json:"success_count"`     // 半开状态下的连续成功计数
	LastFailureTime time.Time `json:"last_failure_time"` // 最近一次失败的时间戳，用于冷却时间计算
}

// Snapshot 返回熔断器当前状态的快照，用于持久化到数据库。
func (cb *CircuitBreaker) Snapshot() CircuitBreakerSnapshot {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerSnapshot{
		State:           cb.state.String(),
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastFailureTime: cb.lastFailureTime,
	}
}

// RestoreFromSnapshot 从持久化的快照恢复熔断器状态。
// 用于服务重启后恢复上次的熔断状态，避免重启导致熔断器被意外重置。
func (cb *CircuitBreaker) RestoreFromSnapshot(snap CircuitBreakerSnapshot) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch snap.State {
	case "open":
		cb.state = StateOpen
	case "half_open":
		cb.state = StateHalfOpen
	default:
		cb.state = StateClosed
	}

	cb.failureCount = snap.FailureCount
	cb.successCount = snap.SuccessCount
	cb.lastFailureTime = snap.LastFailureTime

	cb.logger.Info("circuit breaker state restored",
		zap.String("state", snap.State),
		zap.Int("failure_count", snap.FailureCount),
		zap.Time("last_failure", snap.LastFailureTime),
	)
}
