package biz

import (
	"testing"
	"time"
)

// TestFrequencyCounter_RecordAndCount 验证 Record 记录事件后，Count 能正确返回窗口内的事件计数。
func TestFrequencyCounter_RecordAndCount(t *testing.T) {
	fc := NewFrequencyCounter()

	// 连续记录 3 次事件
	fc.Record("rule-1")
	fc.Record("rule-1")
	fc.Record("rule-1")

	// 在 1 分钟窗口内应统计到 3 次
	count := fc.Count("rule-1", 1*time.Minute)
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

// TestFrequencyCounter_CountEmptyKey 验证查询不存在的键时返回 0。
func TestFrequencyCounter_CountEmptyKey(t *testing.T) {
	fc := NewFrequencyCounter()

	count := fc.Count("nonexistent", 1*time.Minute)
	if count != 0 {
		t.Errorf("expected 0 for unknown key, got %d", count)
	}
}

// TestFrequencyCounter_IndependentKeys 验证不同键之间的事件计数相互独立。
func TestFrequencyCounter_IndependentKeys(t *testing.T) {
	fc := NewFrequencyCounter()

	fc.Record("rule-A")
	fc.Record("rule-A")
	fc.Record("rule-B")

	countA := fc.Count("rule-A", 1*time.Minute)
	countB := fc.Count("rule-B", 1*time.Minute)

	if countA != 2 {
		t.Errorf("rule-A: expected 2, got %d", countA)
	}
	if countB != 1 {
		t.Errorf("rule-B: expected 1, got %d", countB)
	}
}

// TestFrequencyCounter_WindowExpiry 验证超出滑动窗口的事件不被计入，窗口内的事件被正确计入。
func TestFrequencyCounter_WindowExpiry(t *testing.T) {
	// 手动构造计数器，注入过期的时间戳（50ms 前）
	fc := &FrequencyCounter{
		events: make(map[string][]time.Time),
	}

	oldTime := time.Now().Add(-50 * time.Millisecond)
	fc.events["rule-1"] = []time.Time{oldTime, oldTime}

	// 10ms 窗口内应排除 50ms 前的事件
	count := fc.Count("rule-1", 10*time.Millisecond)
	if count != 0 {
		t.Errorf("expected 0 for expired events, got %d", count)
	}

	// 1 秒窗口内应包含这些事件
	count = fc.Count("rule-1", 1*time.Second)
	if count != 2 {
		t.Errorf("expected 2 within 1s window, got %d", count)
	}
}

// TestFrequencyCounter_Cleanup 验证 Cleanup 方法移除超过 1 小时的旧事件，保留近期事件。
func TestFrequencyCounter_Cleanup(t *testing.T) {
	fc := &FrequencyCounter{
		events: make(map[string][]time.Time),
	}

	// 添加 2 小时前的旧事件（应被清理）
	oldTime := time.Now().Add(-2 * time.Hour)
	fc.events["old-rule"] = []time.Time{oldTime, oldTime}

	// 添加近期事件（应被保留）
	fc.events["new-rule"] = []time.Time{time.Now()}

	fc.Cleanup()

	if _, ok := fc.events["old-rule"]; ok {
		t.Error("old-rule should be cleaned up")
	}
	if _, ok := fc.events["new-rule"]; !ok {
		t.Error("new-rule should be kept")
	}
}

// TestFrequencyCounter_CleanupKeepsRecentInMixedKey 验证同一键下混合新旧事件时，
// Cleanup 仅移除过期事件，保留近期事件。
func TestFrequencyCounter_CleanupKeepsRecentInMixedKey(t *testing.T) {
	fc := &FrequencyCounter{
		events: make(map[string][]time.Time),
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now()

	fc.events["mixed"] = []time.Time{oldTime, recentTime}

	fc.Cleanup()

	remaining := fc.events["mixed"]
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining event after cleanup, got %d", len(remaining))
	}
}
