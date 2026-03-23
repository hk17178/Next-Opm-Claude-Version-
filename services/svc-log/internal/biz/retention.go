package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RetentionExecutor 定时执行日志保留策略，根据 retention_policies 表中的规则删除过期的 ES 索引。
// 采用 cron-like goroutine 模式，每隔固定间隔检查并执行清理。
type RetentionExecutor struct {
	logRepo     LogRepository
	esRepo      ESRepository
	logger      *zap.Logger
	interval    time.Duration
	indexPrefix string

	mu      sync.Mutex
	stopped bool
	stopCh  chan struct{}
}

// NewRetentionExecutor 创建保留策略执行器。
// interval 指定检查间隔（建议 1h），indexPrefix 为 ES 索引前缀（如 "opsnexus-log"）。
func NewRetentionExecutor(logRepo LogRepository, esRepo ESRepository, logger *zap.Logger, interval time.Duration, indexPrefix string) *RetentionExecutor {
	return &RetentionExecutor{
		logRepo:     logRepo,
		esRepo:      esRepo,
		logger:      logger,
		interval:    interval,
		indexPrefix: indexPrefix,
		stopCh:      make(chan struct{}),
	}
}

// Start 启动后台定时任务协程，立即执行一次后按固定间隔重复执行。
func (re *RetentionExecutor) Start() {
	go re.run()
}

// Stop 优雅停止定时任务协程。
func (re *RetentionExecutor) Stop() {
	re.mu.Lock()
	defer re.mu.Unlock()
	if !re.stopped {
		re.stopped = true
		close(re.stopCh)
	}
}

func (re *RetentionExecutor) run() {
	re.logger.Info("retention executor started", zap.Duration("interval", re.interval))

	// 启动时立即执行一次
	re.execute()

	ticker := time.NewTicker(re.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			re.execute()
		case <-re.stopCh:
			re.logger.Info("retention executor stopped")
			return
		}
	}
}

// execute 执行一次保留策略检查，遍历所有启用的策略并删除超期索引。
func (re *RetentionExecutor) execute() {
	ctx := context.Background()

	policies, err := re.logRepo.ListRetentionPolicies(ctx)
	if err != nil {
		re.logger.Error("failed to list retention policies", zap.Error(err))
		return
	}

	if len(policies) == 0 {
		return
	}

	// 查询所有匹配的 ES 索引
	pattern := fmt.Sprintf("%s-*", re.indexPrefix)
	indices, err := re.esRepo.ListIndices(ctx, pattern)
	if err != nil {
		re.logger.Error("failed to list ES indices", zap.Error(err))
		return
	}

	now := time.Now()
	deletedCount := 0

	for _, idx := range indices {
		if idx.CreatedAt.IsZero() {
			continue
		}

		ageDays := int(now.Sub(idx.CreatedAt).Hours() / 24)
		maxRetentionDays := re.getMaxRetentionDays(policies)

		if ageDays > maxRetentionDays {
			re.logger.Info("deleting expired index",
				zap.String("index", idx.Name),
				zap.Int("age_days", ageDays),
				zap.Int("max_retention_days", maxRetentionDays),
			)
			if err := re.esRepo.DeleteIndex(ctx, idx.Name); err != nil {
				re.logger.Error("failed to delete index",
					zap.String("index", idx.Name),
					zap.Error(err),
				)
				continue
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		re.logger.Info("retention execution completed",
			zap.Int("deleted_indices", deletedCount),
		)
	}
}

// getMaxRetentionDays 计算所有启用策略中的最大保留天数（hot + warm + cold）。
// 索引在超过所有策略的最大保留期后才会被删除。
func (re *RetentionExecutor) getMaxRetentionDays(policies []*RetentionPolicy) int {
	maxDays := 0
	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		totalDays := p.HotDays + p.WarmDays + p.ColdDays
		if totalDays > maxDays {
			maxDays = totalDays
		}
	}
	if maxDays == 0 {
		maxDays = 365 // fallback default
	}
	return maxDays
}
