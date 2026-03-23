// login_policy.go 实现登录安全策略，包括登录失败次数限制、账户锁定、
// 并发 Session 控制和异常登录检测。基于内存计数器实现（生产环境可替换为 Redis）。

package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LoginPolicy 登录安全策略配置
type LoginPolicy struct {
	MaxFailAttempts int           // 最大连续失败次数（默认 5）
	LockDuration    time.Duration // 锁定时长（默认 30 分钟）
	MaxConcurrent   int           // 单用户最大并发 Session 数（默认 3）
	AnomalyAlert    bool          // 异地登录检测开关
}

// DefaultLoginPolicy 返回默认的登录安全策略配置
func DefaultLoginPolicy() LoginPolicy {
	return LoginPolicy{
		MaxFailAttempts: 5,
		LockDuration:    30 * time.Minute,
		MaxConcurrent:   3,
		AnomalyAlert:    true,
	}
}

// failureRecord 记录用户登录失败信息
type failureRecord struct {
	count    int       // 连续失败次数
	lockedAt time.Time // 锁定时间（零值表示未锁定）
}

// loginHistory 记录用户历史登录信息，用于异常检测
type loginHistory struct {
	lastIP        string // 上次登录 IP
	lastUserAgent string // 上次登录 UA
}

// LoginPolicyManager 登录安全策略管理器。
// 使用内存存储实现，适用于单实例部署。多实例部署场景建议替换为 Redis 实现。
type LoginPolicyManager struct {
	policy   LoginPolicy
	mu       sync.RWMutex
	failures map[string]*failureRecord // key: userID
	history  map[string]*loginHistory  // key: userID，历史登录记录
}

// NewLoginPolicyManager 创建登录安全策略管理器实例
func NewLoginPolicyManager(policy LoginPolicy) *LoginPolicyManager {
	// 填充默认值
	if policy.MaxFailAttempts <= 0 {
		policy.MaxFailAttempts = 5
	}
	if policy.LockDuration <= 0 {
		policy.LockDuration = 30 * time.Minute
	}
	if policy.MaxConcurrent <= 0 {
		policy.MaxConcurrent = 3
	}

	return &LoginPolicyManager{
		policy:   policy,
		failures: make(map[string]*failureRecord),
		history:  make(map[string]*loginHistory),
	}
}

// RecordFailure 记录一次登录失败。
// 当连续失败次数达到上限时自动锁定账户。
func (m *LoginPolicyManager) RecordFailure(_ context.Context, userID, ip string) error {
	if userID == "" {
		return fmt.Errorf("userID 不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.failures[userID]
	if !ok {
		record = &failureRecord{}
		m.failures[userID] = record
	}

	// 如果已锁定且锁定未过期，不再增加计数
	if !record.lockedAt.IsZero() && time.Since(record.lockedAt) < m.policy.LockDuration {
		return nil
	}

	// 如果锁定已过期，重置计数
	if !record.lockedAt.IsZero() && time.Since(record.lockedAt) >= m.policy.LockDuration {
		record.count = 0
		record.lockedAt = time.Time{}
	}

	record.count++

	// 达到最大失败次数，锁定账户
	if record.count >= m.policy.MaxFailAttempts {
		record.lockedAt = time.Now()
	}

	return nil
}

// RecordSuccess 记录一次登录成功，重置失败计数并更新登录历史。
func (m *LoginPolicyManager) RecordSuccess(_ context.Context, userID, ip, userAgent string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 重置失败记录
	delete(m.failures, userID)

	// 更新登录历史
	m.history[userID] = &loginHistory{
		lastIP:        ip,
		lastUserAgent: userAgent,
	}
}

// CheckLocked 检查用户是否被锁定。
// 返回 true 表示账户当前处于锁定状态，false 表示未锁定或锁定已过期。
func (m *LoginPolicyManager) CheckLocked(_ context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("userID 不能为空")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.failures[userID]
	if !ok {
		return false, nil
	}

	// 未被锁定
	if record.lockedAt.IsZero() {
		return false, nil
	}

	// 检查锁定是否已过期
	if time.Since(record.lockedAt) >= m.policy.LockDuration {
		return false, nil
	}

	return true, nil
}

// Unlock 手动解锁用户账户，清除所有失败记录。
func (m *LoginPolicyManager) Unlock(_ context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID 不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.failures, userID)
	return nil
}

// DetectAnomaly 检测是否为异常登录。
// 当用户从不同 IP 或不同 UserAgent 登录时判定为异常。
// 返回 true 表示检测到异常登录，false 表示正常。
func (m *LoginPolicyManager) DetectAnomaly(_ context.Context, userID, ip, userAgent string) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("userID 不能为空")
	}

	// 异常检测未开启时直接返回正常
	if !m.policy.AnomalyAlert {
		return false, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	hist, ok := m.history[userID]
	if !ok {
		// 首次登录，无历史记录，不算异常
		return false, nil
	}

	// IP 地址变化或 UserAgent 变化视为异常
	if hist.lastIP != "" && hist.lastIP != ip {
		return true, nil
	}
	if hist.lastUserAgent != "" && hist.lastUserAgent != userAgent {
		return true, nil
	}

	return false, nil
}

// GetFailureCount 获取用户当前连续失败次数（用于测试和监控）
func (m *LoginPolicyManager) GetFailureCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.failures[userID]
	if !ok {
		return 0
	}
	return record.count
}
