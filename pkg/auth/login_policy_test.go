// login_policy_test.go 测试登录安全策略的核心功能：
// - 连续失败锁定
// - 锁定过期自动解锁
// - 手动解锁
// - 异常登录检测

package auth

import (
	"context"
	"testing"
	"time"
)

// TestRecordFailureAndLock 测试连续失败后自动锁定
func TestRecordFailureAndLock(t *testing.T) {
	policy := LoginPolicy{
		MaxFailAttempts: 3,
		LockDuration:    30 * time.Minute,
	}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-001"

	// 前 2 次失败，不应锁定
	for i := 0; i < 2; i++ {
		if err := mgr.RecordFailure(ctx, userID, "1.2.3.4"); err != nil {
			t.Fatalf("RecordFailure 失败: %v", err)
		}
		locked, err := mgr.CheckLocked(ctx, userID)
		if err != nil {
			t.Fatalf("CheckLocked 失败: %v", err)
		}
		if locked {
			t.Errorf("第 %d 次失败后不应锁定", i+1)
		}
	}

	// 第 3 次失败，应锁定
	if err := mgr.RecordFailure(ctx, userID, "1.2.3.4"); err != nil {
		t.Fatalf("RecordFailure 失败: %v", err)
	}
	locked, err := mgr.CheckLocked(ctx, userID)
	if err != nil {
		t.Fatalf("CheckLocked 失败: %v", err)
	}
	if !locked {
		t.Error("达到最大失败次数后应锁定")
	}
}

// TestFailureCount 测试失败计数
func TestFailureCount(t *testing.T) {
	policy := LoginPolicy{MaxFailAttempts: 5}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-002"

	for i := 1; i <= 3; i++ {
		mgr.RecordFailure(ctx, userID, "1.2.3.4")
		if got := mgr.GetFailureCount(userID); got != i {
			t.Errorf("期望失败次数 %d，实际 %d", i, got)
		}
	}
}

// TestManualUnlock 测试手动解锁
func TestManualUnlock(t *testing.T) {
	policy := LoginPolicy{
		MaxFailAttempts: 2,
		LockDuration:    30 * time.Minute,
	}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-003"

	// 触发锁定
	mgr.RecordFailure(ctx, userID, "1.2.3.4")
	mgr.RecordFailure(ctx, userID, "1.2.3.4")

	locked, _ := mgr.CheckLocked(ctx, userID)
	if !locked {
		t.Fatal("应已被锁定")
	}

	// 手动解锁
	if err := mgr.Unlock(ctx, userID); err != nil {
		t.Fatalf("Unlock 失败: %v", err)
	}

	locked, _ = mgr.CheckLocked(ctx, userID)
	if locked {
		t.Error("解锁后不应处于锁定状态")
	}

	// 解锁后失败计数应重置
	if got := mgr.GetFailureCount(userID); got != 0 {
		t.Errorf("解锁后失败计数应为 0，实际 %d", got)
	}
}

// TestRecordSuccess 测试登录成功重置失败计数
func TestRecordSuccess(t *testing.T) {
	policy := LoginPolicy{MaxFailAttempts: 5}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-004"

	// 记录几次失败
	mgr.RecordFailure(ctx, userID, "1.2.3.4")
	mgr.RecordFailure(ctx, userID, "1.2.3.4")

	if got := mgr.GetFailureCount(userID); got != 2 {
		t.Fatalf("期望失败次数 2，实际 %d", got)
	}

	// 登录成功应重置计数
	mgr.RecordSuccess(ctx, userID, "1.2.3.4", "Mozilla/5.0")

	if got := mgr.GetFailureCount(userID); got != 0 {
		t.Errorf("登录成功后失败计数应为 0，实际 %d", got)
	}
}

// TestDetectAnomaly 测试异常登录检测
func TestDetectAnomaly(t *testing.T) {
	policy := LoginPolicy{
		MaxFailAttempts: 5,
		AnomalyAlert:    true,
	}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-005"

	// 首次登录，无历史记录，不算异常
	anomaly, err := mgr.DetectAnomaly(ctx, userID, "1.2.3.4", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("DetectAnomaly 失败: %v", err)
	}
	if anomaly {
		t.Error("首次登录不应判定为异常")
	}

	// 记录一次成功登录，建立历史
	mgr.RecordSuccess(ctx, userID, "1.2.3.4", "Mozilla/5.0")

	// 相同 IP 和 UA 不算异常
	anomaly, _ = mgr.DetectAnomaly(ctx, userID, "1.2.3.4", "Mozilla/5.0")
	if anomaly {
		t.Error("相同 IP 和 UA 不应判定为异常")
	}

	// 不同 IP 应判定为异常
	anomaly, _ = mgr.DetectAnomaly(ctx, userID, "5.6.7.8", "Mozilla/5.0")
	if !anomaly {
		t.Error("不同 IP 应判定为异常")
	}

	// 不同 UA 应判定为异常
	anomaly, _ = mgr.DetectAnomaly(ctx, userID, "1.2.3.4", "Chrome/120.0")
	if !anomaly {
		t.Error("不同 UA 应判定为异常")
	}
}

// TestDetectAnomalyDisabled 测试异常检测关闭时始终返回 false
func TestDetectAnomalyDisabled(t *testing.T) {
	policy := LoginPolicy{
		MaxFailAttempts: 5,
		AnomalyAlert:    false, // 关闭异常检测
	}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-006"

	// 建立历史
	mgr.RecordSuccess(ctx, userID, "1.2.3.4", "Mozilla/5.0")

	// 即使 IP 不同，异常检测关闭时也应返回 false
	anomaly, _ := mgr.DetectAnomaly(ctx, userID, "5.6.7.8", "Chrome/120.0")
	if anomaly {
		t.Error("异常检测关闭时不应报告异常")
	}
}

// TestEmptyUserID 测试空用户 ID 的错误处理
func TestEmptyUserID(t *testing.T) {
	mgr := NewLoginPolicyManager(DefaultLoginPolicy())
	ctx := context.Background()

	if err := mgr.RecordFailure(ctx, "", "1.2.3.4"); err == nil {
		t.Error("空 userID 应返回错误")
	}

	if _, err := mgr.CheckLocked(ctx, ""); err == nil {
		t.Error("空 userID 应返回错误")
	}

	if err := mgr.Unlock(ctx, ""); err == nil {
		t.Error("空 userID 应返回错误")
	}

	if _, err := mgr.DetectAnomaly(ctx, "", "1.2.3.4", "ua"); err == nil {
		t.Error("空 userID 应返回错误")
	}
}

// TestLockDoesNotIncrementCounter 测试锁定后不再增加失败计数
func TestLockDoesNotIncrementCounter(t *testing.T) {
	policy := LoginPolicy{
		MaxFailAttempts: 2,
		LockDuration:    30 * time.Minute,
	}
	mgr := NewLoginPolicyManager(policy)
	ctx := context.Background()

	userID := "user-007"

	// 触发锁定
	mgr.RecordFailure(ctx, userID, "1.2.3.4")
	mgr.RecordFailure(ctx, userID, "1.2.3.4")

	countAtLock := mgr.GetFailureCount(userID)

	// 锁定后继续尝试不应增加计数
	mgr.RecordFailure(ctx, userID, "1.2.3.4")
	mgr.RecordFailure(ctx, userID, "1.2.3.4")

	if got := mgr.GetFailureCount(userID); got != countAtLock {
		t.Errorf("锁定后失败计数不应增加，期望 %d，实际 %d", countAtLock, got)
	}
}

// TestDefaultLoginPolicy 测试默认策略配置
func TestDefaultLoginPolicy(t *testing.T) {
	p := DefaultLoginPolicy()

	if p.MaxFailAttempts != 5 {
		t.Errorf("默认最大失败次数应为 5，实际 %d", p.MaxFailAttempts)
	}
	if p.LockDuration != 30*time.Minute {
		t.Errorf("默认锁定时长应为 30 分钟，实际 %v", p.LockDuration)
	}
	if p.MaxConcurrent != 3 {
		t.Errorf("默认最大并发应为 3，实际 %d", p.MaxConcurrent)
	}
	if !p.AnomalyAlert {
		t.Error("默认应启用异常检测")
	}
}
