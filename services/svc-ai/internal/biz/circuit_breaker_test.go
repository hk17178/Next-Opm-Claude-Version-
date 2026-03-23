package biz

import (
	"testing"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

// newTestCB 创建用于测试的熔断器实例：失败阈值=3，成功阈值=2，冷却时间=1 秒。
func newTestCB() *CircuitBreaker {
	logger, _ := zap.NewDevelopment()
	return NewCircuitBreaker(config.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		TimeoutSeconds:   1,
	}, logger)
}

// TestCircuitBreaker_InitialState 验证熔断器初始状态为 Closed，且允许请求通过。
func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := newTestCB()
	if cb.State() != StateClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Error("closed breaker should allow requests")
	}
}

// TestCircuitBreaker_OpensAfterThreshold 验证连续失败达到阈值后熔断器切换为 Open 状态。
func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := newTestCB()

	// Record failures up to threshold
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Error("should still be closed before threshold")
	}

	cb.RecordFailure() // 3rd failure = threshold
	if cb.State() != StateOpen {
		t.Errorf("expected open after %d failures, got %s", 3, cb.State())
	}
}

// TestCircuitBreaker_BlocksWhenOpen 验证 Open 状态下（冷却时间内）熔断器拒绝所有请求。
func TestCircuitBreaker_BlocksWhenOpen(t *testing.T) {
	cb := newTestCB()

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.Allow() {
		t.Error("open breaker should block requests (within timeout)")
	}
}

// TestCircuitBreaker_SuccessResetsClosed 验证 Closed 状态下成功调用会重置失败计数器。
func TestCircuitBreaker_SuccessResetsClosed(t *testing.T) {
	cb := newTestCB()

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // Should reset failure count

	// Should need 3 more failures to open
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Error("success should have reset failure count")
	}
}

// TestCircuitBreaker_Snapshot 验证快照功能正确导出熔断器的当前状态（状态、失败计数、时间戳）。
func TestCircuitBreaker_Snapshot(t *testing.T) {
	cb := newTestCB()

	// Open the circuit breaker
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	snap := cb.Snapshot()
	if snap.State != "open" {
		t.Errorf("snapshot state: want open, got %s", snap.State)
	}
	if snap.FailureCount != 3 {
		t.Errorf("snapshot failure_count: want 3, got %d", snap.FailureCount)
	}
	if snap.LastFailureTime.IsZero() {
		t.Error("snapshot last_failure_time should not be zero")
	}
}

// TestCircuitBreaker_RestoreFromSnapshot 验证从快照恢复后熔断器保持 Open 状态并继续阻止请求。
func TestCircuitBreaker_RestoreFromSnapshot(t *testing.T) {
	cb := newTestCB()

	// Open and snapshot
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	snap := cb.Snapshot()

	// Create a new breaker and restore
	cb2 := newTestCB()
	if cb2.State() != StateClosed {
		t.Fatalf("new breaker should be closed, got %s", cb2.State())
	}

	cb2.RestoreFromSnapshot(snap)
	if cb2.State() != StateOpen {
		t.Errorf("restored breaker state: want open, got %s", cb2.State())
	}

	// Restored breaker should block requests (within timeout)
	if cb2.Allow() {
		t.Error("restored open breaker should block requests")
	}
}

// TestCircuitBreaker_RestoreClosedState 验证从 Closed 状态快照恢复后熔断器正常放行请求。
func TestCircuitBreaker_RestoreClosedState(t *testing.T) {
	cb := newTestCB()
	snap := CircuitBreakerSnapshot{
		State:        "closed",
		FailureCount: 0,
		SuccessCount: 0,
	}
	cb.RestoreFromSnapshot(snap)
	if cb.State() != StateClosed {
		t.Errorf("expected closed after restore, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Error("closed restored breaker should allow requests")
	}
}

// TestCircuitBreaker_StateString 验证各熔断器状态的字符串表示（closed/open/half_open/unknown）。
func TestCircuitBreaker_StateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half_open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, tt.state.String(), tt.expected)
		}
	}
}
