// Package biz 提供日志服务的核心业务逻辑，包含保留策略的定时调度功能。
package biz

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// RetentionScheduler 是保留策略的定时调度器，每隔 6 小时自动执行一次保留策略清理。
// 与 RetentionExecutor 的区别：
//   - RetentionExecutor 使用内部 stopCh 进行手动 Stop()，适用于不需要 context 传播的场景。
//   - RetentionScheduler 接受外部 context.Context，可与服务的生命周期 context 集成，
//     在 context 取消时（如服务优雅关闭）自动停止定时任务。
//
// 推荐在 main() 中通过 go scheduler.Start(ctx) 启动，并在关闭阶段 cancel() 上下文以停止。
type RetentionScheduler struct {
	// executor 是实际执行保留策略清理逻辑的执行器实例
	executor *RetentionExecutor
	// interval 是两次清理之间的固定时间间隔，默认 6 小时
	interval time.Duration
	// logger 用于记录调度器的运行状态
	logger *zap.Logger
}

// NewRetentionScheduler 创建保留策略定时调度器。
//
// 参数：
//   - logRepo：日志元数据仓储，用于加载 retention_policies 配置。
//   - esRepo：Elasticsearch 仓储，用于列举和删除过期索引。
//   - logger：结构化日志记录器。
//   - indexPrefix：ES 索引名称前缀，如 "opsnexus-log"。
//
// 返回：已配置好 6 小时间隔的调度器实例，调用 Start(ctx) 启动。
func NewRetentionScheduler(
	logRepo LogRepository,
	esRepo ESRepository,
	logger *zap.Logger,
	indexPrefix string,
) *RetentionScheduler {
	// 内部创建一个固定 6 小时间隔的 RetentionExecutor 实例
	executor := NewRetentionExecutor(logRepo, esRepo, logger, 6*time.Hour, indexPrefix)
	return &RetentionScheduler{
		executor: executor,
		interval: 6 * time.Hour,
		logger:   logger,
	}
}

// NewRetentionSchedulerWithInterval 创建自定义间隔的保留策略定时调度器，主要供测试使用。
//
// 参数：
//   - logRepo：日志元数据仓储。
//   - esRepo：Elasticsearch 仓储。
//   - logger：结构化日志记录器。
//   - interval：定时执行间隔（生产环境建议使用默认的 6h，测试时可使用更短间隔）。
//   - indexPrefix：ES 索引名称前缀。
//
// 返回：已配置好指定间隔的调度器实例。
func NewRetentionSchedulerWithInterval(
	logRepo LogRepository,
	esRepo ESRepository,
	logger *zap.Logger,
	interval time.Duration,
	indexPrefix string,
) *RetentionScheduler {
	executor := NewRetentionExecutor(logRepo, esRepo, logger, interval, indexPrefix)
	return &RetentionScheduler{
		executor: executor,
		interval: interval,
		logger:   logger,
	}
}

// Start 在当前 goroutine 中启动定时调度循环，阻塞直到 ctx 被取消。
// 调用方应使用 go scheduler.Start(ctx) 在独立 goroutine 中运行。
//
// 执行策略：
//  1. 启动后立即执行一次保留策略清理（无需等待第一个 tick）。
//  2. 之后每隔 interval（默认 6h）执行一次清理。
//  3. ctx.Done() 触发后，记录日志并返回，调用方的 defer/cancel 负责资源清理。
//
// 参数：
//   - ctx：外部控制的上下文，取消时调度器优雅退出（通常来自 main 的 cancel）。
func (s *RetentionScheduler) Start(ctx context.Context) {
	s.logger.Info("retention scheduler started",
		zap.Duration("interval", s.interval),
	)

	// 启动时立即执行一次，确保服务重启后第一时间清理过期数据
	s.runOnce(ctx)

	// 使用 time.NewTicker 按固定间隔触发后续的清理任务
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 定时触发：执行一次保留策略清理
			s.runOnce(ctx)

		case <-ctx.Done():
			// context 已取消（服务关闭或父 ctx 超时），退出调度循环
			s.logger.Info("retention scheduler stopping",
				zap.String("reason", ctx.Err().Error()),
			)
			return
		}
	}
}

// runOnce 以安全方式执行一次保留策略清理。
// 检查 ctx 是否已取消，若已取消则跳过本次执行，避免在关闭过程中发起不必要的 ES 请求。
//
// 参数：
//   - ctx：用于提前中止清理的上下文（当前仅做取消检查，清理操作内部使用 context.Background）。
func (s *RetentionScheduler) runOnce(ctx context.Context) {
	// 如果 context 已取消，跳过本次执行
	select {
	case <-ctx.Done():
		return
	default:
	}

	s.logger.Info("retention scheduler: running cleanup cycle")
	// 委托给 RetentionExecutor 执行实际的索引清理逻辑
	s.executor.execute()
}
