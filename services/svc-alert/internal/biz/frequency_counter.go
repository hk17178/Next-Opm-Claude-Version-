package biz

import (
	"sync"
	"time"
)

// FrequencyCounter 实现内存中的滑动窗口事件计数器，用于 Layer 1 频率规则告警。
// 每个键维护一个事件时间戳列表，Count 方法返回指定窗口内的事件数量。
type FrequencyCounter struct {
	mu     sync.Mutex
	events map[string][]time.Time
}

// NewFrequencyCounter 创建频率计数器并启动后台清理协程。
func NewFrequencyCounter() *FrequencyCounter {
	fc := &FrequencyCounter{
		events: make(map[string][]time.Time),
	}
	go fc.cleanupLoop()
	return fc
}

// Record 将当前时间戳追加到指定键的事件列表中。
func (fc *FrequencyCounter) Record(key string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.events[key] = append(fc.events[key], time.Now())
}

// Count 返回指定键在给定时间窗口内（从当前时间向前计算）的事件数量。
func (fc *FrequencyCounter) Count(key string, window time.Duration) int {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	timestamps, ok := fc.events[key]
	if !ok {
		return 0
	}

	cutoff := time.Now().Add(-window)
	count := 0
	for _, ts := range timestamps {
		if !ts.Before(cutoff) {
			count++
		}
	}
	return count
}

// Cleanup 移除所有超过 1 小时的事件时间戳，防止内存无限增长。
func (fc *FrequencyCounter) Cleanup() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for key, timestamps := range fc.events {
		kept := timestamps[:0]
		for _, ts := range timestamps {
			if !ts.Before(cutoff) {
				kept = append(kept, ts)
			}
		}
		if len(kept) == 0 {
			delete(fc.events, key)
		} else {
			fc.events[key] = kept
		}
	}
}

// cleanupLoop 每 10 分钟在后台运行一次 Cleanup。
func (fc *FrequencyCounter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		fc.Cleanup()
	}
}
