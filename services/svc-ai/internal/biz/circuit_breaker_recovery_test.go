package biz

import (
	"testing"
	"time"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

// newShortTimeoutCB 创建一个用于测试的短超时熔断器实例。
// 配置：失败阈值 2 次、成功恢复阈值 2 次、超时 0 秒（立即超时，便于测试状态转换）。
func newShortTimeoutCB() *CircuitBreaker {
	logger, _ := zap.NewDevelopment()
	return NewCircuitBreaker(config.CircuitBreakerConfig{
		FailureThreshold: 2, // 连续失败 2 次触发熔断
		SuccessThreshold: 2, // 半开状态下连续成功 2 次恢复
		TimeoutSeconds:   0, // 0 秒超时，测试中立即从 Open 转为 HalfOpen
	}, logger)
}

// TestCircuitBreaker_OpenToHalfOpenAfterTimeout 验证熔断器从 Open 状态超时后自动转为 HalfOpen 状态。
// 流程：连续失败触发熔断 → 等待超时 → Allow() 调用触发状态转换为 HalfOpen。
func TestCircuitBreaker_OpenToHalfOpenAfterTimeout(t *testing.T) {
	cb := newShortTimeoutCB()

	// 连续 2 次失败触发熔断，进入 Open 状态
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatal("期望进入 Open 状态")
	}

	// 超时时间为 0 秒，等待 1ms 确保 time.Since > 0，Allow() 应触发转为 HalfOpen
	time.Sleep(1 * time.Millisecond)
	if !cb.Allow() {
		t.Error("超时后 Allow() 应返回 true（转换为 HalfOpen 状态）")
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("期望 HalfOpen 状态，实际为 %s", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenToClosedAfterSuccessThreshold 验证 HalfOpen 状态下连续成功达到阈值后恢复为 Closed。
// 流程：熔断 → 超时 → HalfOpen → 连续成功 2 次 → Closed。
func TestCircuitBreaker_HalfOpenToClosedAfterSuccessThreshold(t *testing.T) {
	cb := newShortTimeoutCB()

	// 触发熔断并等待超时，进入 HalfOpen 状态
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(1 * time.Millisecond)
	cb.Allow() // 触发 Open → HalfOpen 转换

	if cb.State() != StateHalfOpen {
		t.Fatalf("期望 HalfOpen 状态，实际为 %s", cb.State())
	}

	// 第 1 次成功：未达恢复阈值（2 次），仍保持 HalfOpen
	cb.RecordSuccess()
	if cb.State() != StateHalfOpen {
		t.Error("1 次成功后仍应保持 HalfOpen 状态（恢复阈值为 2）")
	}

	// 第 2 次成功：达到恢复阈值，应恢复为 Closed
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("连续 2 次成功后期望恢复为 Closed 状态，实际为 %s", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenToOpenOnFailure 验证 HalfOpen 状态下任意一次失败立即重新熔断。
// 流程：熔断 → 超时 → HalfOpen → 失败 → 重新 Open。
func TestCircuitBreaker_HalfOpenToOpenOnFailure(t *testing.T) {
	cb := newShortTimeoutCB()

	// 触发熔断并等待超时，进入 HalfOpen 状态
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(1 * time.Millisecond)
	cb.Allow()

	if cb.State() != StateHalfOpen {
		t.Fatalf("期望 HalfOpen 状态，实际为 %s", cb.State())
	}

	// HalfOpen 状态下任意失败应立即重新熔断
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("HalfOpen 状态下失败后期望重新 Open，实际为 %s", cb.State())
	}
}

// TestCircuitBreaker_IsOpenFalseAfterTimeout 验证超时后 IsOpen() 返回 false。
// 当熔断超时时间已过，IsOpen() 应返回 false 表示可以尝试探测请求。
func TestCircuitBreaker_IsOpenFalseAfterTimeout(t *testing.T) {
	cb := newShortTimeoutCB()

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	// 等待超时后，IsOpen() 应返回 false（表示超时已过，可探测）
	time.Sleep(1 * time.Millisecond)
	if cb.IsOpen() {
		t.Error("超时后 IsOpen() 应返回 false")
	}
}

// TestCircuitBreaker_FullRecoveryLifecycle 验证熔断器完整生命周期：
// Closed（正常）→ Open（熔断）→ HalfOpen（探测）→ Closed（恢复）→ 验证计数器重置。
func TestCircuitBreaker_FullRecoveryLifecycle(t *testing.T) {
	cb := newShortTimeoutCB()

	// 阶段 1：初始状态为 Closed，正常处理请求
	if cb.State() != StateClosed {
		t.Fatal("初始状态应为 Closed")
	}
	cb.RecordSuccess()
	cb.RecordSuccess()

	// 阶段 2：连续失败触发熔断，进入 Open 状态
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatal("连续失败后应进入 Open 状态")
	}

	// 阶段 3：等待超时后通过 Allow() 转为 HalfOpen 探测状态
	time.Sleep(1 * time.Millisecond)
	cb.Allow()
	if cb.State() != StateHalfOpen {
		t.Fatal("超时后应转为 HalfOpen 状态")
	}

	// 阶段 4：探测成功，连续 2 次成功恢复为 Closed
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatal("探测成功后应恢复为 Closed 状态")
	}

	// 阶段 5：验证失败计数器已重置，单次失败不应触发熔断
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Error("恢复后单次失败不应触发熔断（计数器已重置）")
	}
}

// TestCircuitBreaker_HalfOpenAllowsRequests 验证 HalfOpen 状态下 Allow() 始终返回 true，
// 允许探测请求通过以检测后端服务是否恢复。
func TestCircuitBreaker_HalfOpenAllowsRequests(t *testing.T) {
	cb := newShortTimeoutCB()

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(1 * time.Millisecond)

	// 第一次 Allow() 触发 Open → HalfOpen 转换，应返回 true
	if !cb.Allow() {
		t.Error("超时后首次 Allow() 应返回 true")
	}

	// HalfOpen 状态下后续 Allow() 也应返回 true，允许探测请求
	if !cb.Allow() {
		t.Error("HalfOpen 状态下 Allow() 应返回 true")
	}
}
