package biz

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Deduplicator 基于指纹的告警去重器，在可配置的冷却窗口内抑制重复告警。
// 按 FR-03-016 规范：相同指纹的告警在解决前被抑制，冷却窗口可按规则配置。
type Deduplicator struct {
	mu     sync.RWMutex
	seen   map[string]time.Time
	window time.Duration
}

// NewDeduplicator 创建去重器，window 指定冷却窗口时长，并启动后台过期清理协程。
func NewDeduplicator(window time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen:   make(map[string]time.Time),
		window: window,
	}
	go d.cleanup()
	return d
}

// Fingerprint 根据规则 ID 和标签集生成稳定的 SHA256 前缀哈希。
// 格式为 "sha256:<hex16>"（取前 16 字节），符合 ON-003 规范。
// 标签键排序后拼接，确保相同规则+相同标签组合始终生成相同指纹。
func Fingerprint(ruleID string, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(ruleID)
	b.WriteByte('|')
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}

	hash := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("sha256:%x", hash[:16])
}

// IsDuplicate 检查该指纹是否在去重窗口内已被记录过。
func (d *Deduplicator) IsDuplicate(fingerprint string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	lastSeen, exists := d.seen[fingerprint]
	if !exists {
		return false
	}
	return time.Since(lastSeen) < d.window
}

// Record 将指纹标记为在当前时间已触发。
func (d *Deduplicator) Record(fingerprint string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[fingerprint] = time.Now()
}

// cleanup 定期清理过期的指纹记录，清理间隔等于去重窗口时长。
func (d *Deduplicator) cleanup() {
	ticker := time.NewTicker(d.window)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for fp, ts := range d.seen {
			if now.Sub(ts) >= d.window {
				delete(d.seen, fp)
			}
		}
		d.mu.Unlock()
	}
}
