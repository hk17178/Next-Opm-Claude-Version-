package biz

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// EscalationPolicy 定义按严重等级划分的超时升级规则。
type EscalationPolicy struct {
	Severity          string        // 适用的严重等级："P0"/"P1"/"P2"/"P3"
	AckTimeout        time.Duration // 确认超时：超过该时长未确认则自动升级
	ResolutionTimeout time.Duration // 解决超时：确认后超过该时长未解决则自动升级
	EscalateTo        string        // 升级目标等级（相同等级表示仅发通知不升级）
}

// DefaultEscalationPolicies 返回按 ON-002 FR-06-008 规范定义的默认升级策略。
// P0/致命：15 分钟内确认，4 小时内解决
// P1/严重：30 分钟内确认，8 小时内解决
// P2/重要：2 小时内确认，24 小时内解决
// P3/次要：8 小时内确认，72 小时内解决
func DefaultEscalationPolicies() []EscalationPolicy {
	return []EscalationPolicy{
		{
			Severity:          SeverityP0,
			AckTimeout:        15 * time.Minute,
			ResolutionTimeout: 4 * time.Hour,
			EscalateTo:        SeverityP0, // 已是最高等级，仅触发通知不再升级
		},
		{
			Severity:          SeverityP1,
			AckTimeout:        30 * time.Minute,
			ResolutionTimeout: 8 * time.Hour,
			EscalateTo:        SeverityP0,
		},
		{
			Severity:          SeverityP2,
			AckTimeout:        2 * time.Hour,
			ResolutionTimeout: 24 * time.Hour,
			EscalateTo:        SeverityP1,
		},
		{
			Severity:          SeverityP3,
			AckTimeout:        8 * time.Hour,
			ResolutionTimeout: 72 * time.Hour,
			EscalateTo:        SeverityP2,
		},
	}
}

// EscalationWatcher 后台守护协程，定期检查未及时确认或解决的事件并自动升级。
type EscalationWatcher struct {
	repo     IncidentRepo
	uc       *IncidentUsecase
	policies []EscalationPolicy
	interval time.Duration
	logger   *zap.Logger
}

// NewEscalationWatcher 创建升级监控器实例，使用默认策略和 5 分钟检查间隔。
func NewEscalationWatcher(repo IncidentRepo, uc *IncidentUsecase, logger *zap.Logger) *EscalationWatcher {
	return &EscalationWatcher{
		repo:     repo,
		uc:       uc,
		policies: DefaultEscalationPolicies(),
		interval: 5 * time.Minute,
		logger:   logger.Named("escalation-watcher"),
	}
}

// Start 启动后台定时任务，应以 go watcher.Start(ctx) 方式调用，通过 context 取消停止。
func (w *EscalationWatcher) Start(ctx context.Context) {
	w.logger.Info("escalation watcher started", zap.Duration("interval", w.interval))
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("escalation watcher stopped")
			return
		case <-ticker.C:
			w.checkAndEscalate(ctx)
		}
	}
}

// checkAndEscalate 为定时任务的核心逻辑，遍历所有未关闭事件检查是否超时。
func (w *EscalationWatcher) checkAndEscalate(ctx context.Context) {
	incidents, _, err := w.repo.List(ctx, ListFilter{})
	if err != nil {
		w.logger.Error("failed to list incidents for escalation check", zap.Error(err))
		return
	}

	now := time.Now()

	for _, inc := range incidents {
		// 跳过已解决和已关闭的终态事件
		if inc.Status == StatusResolved || inc.Status == StatusClosed {
			continue
		}

		policy := w.findPolicy(inc.Severity)
		if policy == nil {
			continue
		}

		// 检查确认超时：发现时间 + 确认超时阈值 < 当前时间，且尚未确认
		if inc.AcknowledgedAt == nil && now.After(inc.DetectedAt.Add(policy.AckTimeout)) {
			w.logger.Warn("auto-escalating incident: ack timeout exceeded",
				zap.String("incident_id", inc.IncidentID),
				zap.String("severity", inc.Severity),
				zap.String("escalate_to", policy.EscalateTo),
				zap.Duration("ack_timeout", policy.AckTimeout),
			)
			if _, err := w.uc.EscalateIncident(ctx, inc.IncidentID, policy.EscalateTo, "auto-escalation: ack timeout exceeded"); err != nil {
				w.logger.Error("failed to auto-escalate incident (ack timeout)",
					zap.String("incident_id", inc.IncidentID),
					zap.Error(err),
				)
			}
			continue
		}

		// 检查解决超时：确认时间 + 解决超时阈值 < 当前时间，且尚未解决
		if inc.AcknowledgedAt != nil && inc.ResolvedAt == nil &&
			now.After(inc.AcknowledgedAt.Add(policy.ResolutionTimeout)) {
			w.logger.Warn("auto-escalating incident: resolution timeout exceeded",
				zap.String("incident_id", inc.IncidentID),
				zap.String("severity", inc.Severity),
				zap.String("escalate_to", policy.EscalateTo),
				zap.Duration("resolution_timeout", policy.ResolutionTimeout),
			)
			if _, err := w.uc.EscalateIncident(ctx, inc.IncidentID, policy.EscalateTo, "auto-escalation: resolution timeout exceeded"); err != nil {
				w.logger.Error("failed to auto-escalate incident (resolution timeout)",
					zap.String("incident_id", inc.IncidentID),
					zap.Error(err),
				)
			}
		}
	}
}

// findPolicy 根据严重等级查找匹配的升级策略，未找到时返回 nil。
func (w *EscalationWatcher) findPolicy(severity string) *EscalationPolicy {
	for i := range w.policies {
		if w.policies[i].Severity == severity {
			return &w.policies[i]
		}
	}
	return nil
}
