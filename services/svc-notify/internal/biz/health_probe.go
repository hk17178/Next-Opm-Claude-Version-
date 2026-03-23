package biz

import (
	"context"
	"time"

	"github.com/opsnexus/svc-notify/internal/config"
	"go.uber.org/zap"
)

// ChannelHealthProbe 定期探测每个已启用机器人渠道的连通性，并更新其健康状态。
// 连续失败次数超过阈值（FailureThreshold）的渠道会被自动标记为 "down"，用于降级处理。
// 状态流转：healthy → degraded（单次失败）→ down（达到阈值）→ healthy（探测恢复）。
type ChannelHealthProbe struct {
	botRepo        BotRepo                    // 机器人数据仓储，用于查询和更新健康状态
	channelManager *ChannelManager            // 渠道管理器，用于执行连通性测试
	cfg            config.ChannelHealthConfig  // 健康探测配置（探测间隔、失败阈值）
	logger         *zap.Logger                // 日志记录器
	cancelFn       context.CancelFunc         // 用于停止探测协程的取消函数
}

// NewChannelHealthProbe 创建渠道健康探测器实例。
func NewChannelHealthProbe(
	botRepo BotRepo,
	channelManager *ChannelManager,
	cfg config.ChannelHealthConfig,
	logger *zap.Logger,
) *ChannelHealthProbe {
	return &ChannelHealthProbe{
		botRepo:        botRepo,
		channelManager: channelManager,
		cfg:            cfg,
		logger:         logger,
	}
}

// Start 启动渠道健康探测循环，按配置的间隔（默认 60 秒）定期探测所有已启用渠道。
// 调用方应在 goroutine 中启动，通过 Stop() 或上下文取消来终止。
func (p *ChannelHealthProbe) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	interval := p.cfg.ProbeInterval
	if interval == 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	p.logger.Info("channel health probe started", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.probeAll(ctx)
		}
	}
}

// probeAll 遍历所有已启用的机器人，逐个执行渠道连通性测试。
// 测试失败时累加失败计数，达到阈值标记为 "down"；测试成功则恢复为 "healthy"。
func (p *ChannelHealthProbe) probeAll(ctx context.Context) {
	bots, err := p.botRepo.ListEnabled(ctx)
	if err != nil {
		p.logger.Error("failed to list bots for health probe", zap.Error(err))
		return
	}

	for _, bot := range bots {
		err := p.channelManager.Test(ctx, bot)
		if err != nil {
			newCount := bot.FailureCount + 1
			status := "degraded"
			if newCount >= p.cfg.FailureThreshold {
				status = "down"
				p.logger.Warn("channel marked as down",
					zap.String("bot", bot.Name),
					zap.String("channel", string(bot.ChannelType)),
					zap.Int("failure_count", newCount),
				)
			}
			_ = p.botRepo.UpdateHealthStatus(ctx, bot.ID, status, newCount)
		} else {
			if bot.HealthStatus != "healthy" {
				p.logger.Info("channel recovered",
					zap.String("bot", bot.Name),
					zap.String("channel", string(bot.ChannelType)),
				)
			}
			_ = p.botRepo.UpdateHealthStatus(ctx, bot.ID, "healthy", 0)
		}
	}
}

// Stop 停止渠道健康探测循环。
func (p *ChannelHealthProbe) Stop() {
	if p.cancelFn != nil {
		p.cancelFn()
	}
	p.logger.Info("channel health probe stopped")
}
