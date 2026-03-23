// Package biz 定义通知服务的核心业务模型与领域逻辑，包括通知渠道、机器人、广播规则和去重引擎。
package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/opsnexus/svc-notify/internal/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DedupEngine 使用 Redis 指纹去重来防止重复通知。
// 根据（渠道类型、接收人、事件节点、关键内容）计算内容哈希，
// 在去重时间窗口内相同哈希的通知将被抑制。
type DedupEngine struct {
	rdb    *redis.Client
	cfg    config.DedupConfig
	logger *zap.Logger
}

// NewDedupEngine 创建去重引擎实例。
func NewDedupEngine(rdb *redis.Client, cfg config.DedupConfig, logger *zap.Logger) *DedupEngine {
	return &DedupEngine{
		rdb:    rdb,
		cfg:    cfg,
		logger: logger,
	}
}

// DedupKey 根据渠道类型、接收人、事件节点和内容关键字生成去重指纹哈希（SHA256 前 16 字节）。
func DedupKey(channelType ChannelType, recipient, eventNode, contentKey string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s", channelType, recipient, eventNode, contentKey)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:16])
}

// IsDuplicate 检查该通知是否已在去重窗口内发送过。
// 返回 true 表示是重复通知（应被抑制）。
func (d *DedupEngine) IsDuplicate(ctx context.Context, hash string) (bool, error) {
	key := "notify:dedup:" + hash
	exists, err := d.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("dedup check: %w", err)
	}
	return exists > 0, nil
}

// MarkSent 标记通知哈希为已发送，设置去重窗口 TTL（默认 5 分钟）。
func (d *DedupEngine) MarkSent(ctx context.Context, hash string) error {
	key := "notify:dedup:" + hash
	window := d.cfg.Window
	if window == 0 {
		window = 5 * time.Minute
	}
	return d.rdb.Set(ctx, key, "1", window).Err()
}

// IncrementMergeCount 跟踪同一哈希下合并的通知数量。
// 返回递增后的当前计数，首次递增时自动设置 TTL。
func (d *DedupEngine) IncrementMergeCount(ctx context.Context, hash string) (int64, error) {
	key := "notify:merge:" + hash
	count, err := d.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		window := d.cfg.Window
		if window == 0 {
			window = 5 * time.Minute
		}
		d.rdb.Expire(ctx, key, window)
	}
	return count, nil
}

// ShouldMerge 检查该通知是否应该被合并（合并计数 < 合并上限，默认上限为 10）。
func (d *DedupEngine) ShouldMerge(ctx context.Context, hash string) (bool, int64, error) {
	key := "notify:merge:" + hash
	count, err := d.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}

	limit := d.cfg.MergeLimit
	if limit == 0 {
		limit = 10
	}
	return count < int64(limit), count, nil
}
