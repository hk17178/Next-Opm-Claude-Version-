// Package biz 定义通知服务的核心业务模型与领域逻辑，包括通知渠道、机器人、广播规则和去重引擎。
package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opsnexus/svc-notify/internal/config"
	"go.uber.org/zap"
)

// Broadcaster 处理事件生命周期广播，遍历所有匹配的机器人和规则发送通知。
// 实现 7 节点生命周期通知机制，支持基于严重级别的频率控制。
type Broadcaster struct {
	botRepo        BotRepo
	ruleRepo       BroadcastRuleRepo
	channelManager *ChannelManager
	dedupEngine    *DedupEngine
	logRepo        NotificationLogRepo
	cfg            config.BroadcastConfig
	logger         *zap.Logger
}

// NewBroadcaster 创建广播器实例。
func NewBroadcaster(
	botRepo BotRepo,
	ruleRepo BroadcastRuleRepo,
	channelManager *ChannelManager,
	dedupEngine *DedupEngine,
	logRepo NotificationLogRepo,
	cfg config.BroadcastConfig,
	logger *zap.Logger,
) *Broadcaster {
	return &Broadcaster{
		botRepo:        botRepo,
		ruleRepo:       ruleRepo,
		channelManager: channelManager,
		dedupEngine:    dedupEngine,
		logRepo:        logRepo,
		cfg:            cfg,
		logger:         logger,
	}
}

// Broadcast 将生命周期事件通知发送到所有匹配的机器人，包含严重级别过滤、去重和日志记录。
func (b *Broadcaster) Broadcast(ctx context.Context, event BroadcastEvent) error {
	rules, err := b.ruleRepo.ListByNode(ctx, string(event.Node))
	if err != nil {
		return fmt.Errorf("list broadcast rules: %w", err)
	}

	if len(rules) == 0 {
		b.logger.Debug("no broadcast rules for node", zap.String("node", string(event.Node)))
		return nil
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Check severity filter
		if !b.matchesSeverity(rule, event.Severity) {
			continue
		}

		bot, err := b.botRepo.GetByID(ctx, rule.BotID)
		if err != nil {
			b.logger.Warn("bot not found for rule", zap.String("rule_id", rule.ID.String()), zap.Error(err))
			continue
		}
		if !bot.Enabled || bot.HealthStatus == "down" {
			b.logger.Debug("skipping disabled/down bot", zap.String("bot", bot.Name))
			continue
		}

		// Dedup check
		dedupHash := DedupKey(bot.ChannelType, bot.Name, string(event.Node), event.Title)
		isDup, err := b.dedupEngine.IsDuplicate(ctx, dedupHash)
		if err != nil {
			b.logger.Warn("dedup check failed", zap.Error(err))
		}
		if isDup {
			b.logger.Debug("notification suppressed (duplicate)",
				zap.String("bot", bot.Name),
				zap.String("node", string(event.Node)),
			)
			b.logNotification(ctx, bot, event, StatusSuppressed, dedupHash, "")
			continue
		}

		// Build message content
		msg := b.buildMessage(event, rule)

		// Send
		err = b.channelManager.Send(ctx, bot, msg, nil)
		if err != nil {
			b.logger.Error("broadcast send failed",
				zap.String("bot", bot.Name),
				zap.String("node", string(event.Node)),
				zap.Error(err),
			)
			b.logNotification(ctx, bot, event, StatusFailed, dedupHash, err.Error())
			continue
		}

		// Mark as sent for dedup
		_ = b.dedupEngine.MarkSent(ctx, dedupHash)
		b.logNotification(ctx, bot, event, StatusSent, dedupHash, "")

		b.logger.Info("broadcast sent",
			zap.String("bot", bot.Name),
			zap.String("channel", string(bot.ChannelType)),
			zap.String("node", string(event.Node)),
		)
	}

	return nil
}

// matchesSeverity 检查广播规则的严重级别过滤条件是否匹配当前事件的严重级别。
// 过滤器为空或解析失败时默认匹配所有级别。
func (b *Broadcaster) matchesSeverity(rule *BroadcastRule, severity string) bool {
	if rule.SeverityFilter == nil {
		return true
	}

	var filter []string
	if err := json.Unmarshal(rule.SeverityFilter, &filter); err != nil {
		return true
	}

	if len(filter) == 0 {
		return true
	}

	for _, s := range filter {
		if s == severity {
			return true
		}
	}
	return false
}

// buildMessage 根据事件和规则构建通知消息内容，支持模板变量替换。
func (b *Broadcaster) buildMessage(event BroadcastEvent, rule *BroadcastRule) MessageContent {
	body := event.Body
	if body == "" {
		body = fmt.Sprintf("[%s] %s\nSeverity: %s\nTime: %s",
			event.Node, event.Title, event.Severity, time.Now().Format("2006-01-02 15:04:05"))
	}

	if rule.Template != "" {
		// Simple variable substitution in template
		body = rule.Template
		for k, v := range event.ExtraVars {
			body = replaceVar(body, k, v)
		}
		body = replaceVar(body, "title", event.Title)
		body = replaceVar(body, "severity", event.Severity)
		body = replaceVar(body, "node", string(event.Node))
		body = replaceVar(body, "time", time.Now().Format("2006-01-02 15:04:05"))
	}

	return MessageContent{
		Subject: fmt.Sprintf("[OpsNexus][%s] %s", event.Severity, event.Title),
		Body:    body,
	}
}

// replaceVar 替换模板中的 ${key} 变量为实际值。
func replaceVar(template, key, value string) string {
	return replaceAll(template, "${"+key+"}", value)
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// logNotification 将广播通知的发送结果记录到通知日志表中。
func (b *Broadcaster) logNotification(ctx context.Context, bot *Bot, event BroadcastEvent, status NotifyStatus, hash, errMsg string) {
	log := &NotificationLog{
		ID:             uuid.New(),
		BotID:          &bot.ID,
		ChannelType:    bot.ChannelType,
		MessageType:    "broadcast",
		EventNode:      string(event.Node),
		IncidentID:     event.IncidentID,
		AlertID:        event.AlertID,
		ContentHash:    hash,
		ContentPreview: truncatePreview(event.Title, 200),
		Status:         status,
		ErrorMessage:   errMsg,
		SentAt:         time.Now(),
	}
	if err := b.logRepo.Create(ctx, log); err != nil {
		b.logger.Warn("failed to log notification", zap.Error(err))
	}
}

// truncatePreview 将字符串截断到指定最大长度，超出部分用 "..." 替代。
func truncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StartPeriodicBroadcast 启动基于严重级别的定期重复广播协程。
// P0 每 5 分钟、P1 每 10 分钟，以此类推。当前为占位实现。
func (b *Broadcaster) StartPeriodicBroadcast(ctx context.Context) {
	b.logger.Info("periodic broadcast scheduler started")
	// This would be implemented with a ticker checking active incidents
	// and re-broadcasting based on severity intervals.
	// For now, this is a placeholder that would integrate with the incident state store.
	<-ctx.Done()
}

// HandleAlertFired 处理 alert.fired CloudEvent 事件，将告警级别映射为 P0-P3 后触发广播。
func (b *Broadcaster) HandleAlertFired(ctx context.Context, data json.RawMessage) error {
	var event struct {
		Data struct {
			AlertID     string `json:"alert_id"`
			Title       string `json:"title"`
			Severity    string `json:"severity"`
			HostID      string `json:"host_id"`
			ServiceName string `json:"service_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	severity := mapAlertSeverity(event.Data.Severity)
	alertID, _ := uuid.Parse(event.Data.AlertID)

	return b.Broadcast(ctx, BroadcastEvent{
		Node:     NodeAlertFired,
		Severity: severity,
		AlertID:  &alertID,
		Title:    event.Data.Title,
		Body: fmt.Sprintf("**Alert Fired**\nTitle: %s\nSeverity: %s\nHost: %s\nService: %s",
			event.Data.Title, event.Data.Severity, event.Data.HostID, event.Data.ServiceName),
	})
}

// HandleIncidentCreated 处理 incident.created CloudEvent 事件并触发广播。
func (b *Broadcaster) HandleIncidentCreated(ctx context.Context, data json.RawMessage) error {
	var event struct {
		Data struct {
			IncidentID string `json:"incident_id"`
			Title      string `json:"title"`
			Severity   string `json:"severity"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	incidentID, _ := uuid.Parse(event.Data.IncidentID)

	return b.Broadcast(ctx, BroadcastEvent{
		Node:       NodeIncidentCreated,
		Severity:   event.Data.Severity,
		IncidentID: &incidentID,
		Title:      event.Data.Title,
		Body: fmt.Sprintf("**Incident Created**\nTitle: %s\nSeverity: %s\nStatus: %s",
			event.Data.Title, event.Data.Severity, event.Data.Status),
	})
}

// HandleAIAnalysisDone 处理 ai.analysis.done CloudEvent 事件并触发广播，默认使用 P2 级别。
func (b *Broadcaster) HandleAIAnalysisDone(ctx context.Context, data json.RawMessage) error {
	var event struct {
		Data struct {
			AnalysisID   string `json:"analysis_id"`
			AnalysisType string `json:"analysis_type"`
			Status       string `json:"status"`
			IncidentID   string `json:"incident_id"`
			Result       struct {
				Summary    string  `json:"summary"`
				Confidence float64 `json:"confidence"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	var incidentID *uuid.UUID
	if event.Data.IncidentID != "" {
		id, _ := uuid.Parse(event.Data.IncidentID)
		incidentID = &id
	}

	confidencePct := int(event.Data.Result.Confidence * 100)

	return b.Broadcast(ctx, BroadcastEvent{
		Node:       NodeAIAnalysisDone,
		Severity:   "P2",
		IncidentID: incidentID,
		Title:      fmt.Sprintf("AI Analysis Complete (%d%% confidence)", confidencePct),
		Body: fmt.Sprintf("**AI Analysis Done**\nType: %s\nStatus: %s\nSummary: %s\nConfidence: %d%%",
			event.Data.AnalysisType, event.Data.Status, event.Data.Result.Summary, confidencePct),
	})
}

// HandleIncidentResolved 处理 incident.resolved CloudEvent 事件并触发广播。
func (b *Broadcaster) HandleIncidentResolved(ctx context.Context, data json.RawMessage) error {
	var event struct {
		Data struct {
			IncidentID string `json:"incident_id"`
			Title      string `json:"title"`
			Severity   string `json:"severity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	incidentID, _ := uuid.Parse(event.Data.IncidentID)

	return b.Broadcast(ctx, BroadcastEvent{
		Node:       NodeIncidentResolved,
		Severity:   event.Data.Severity,
		IncidentID: &incidentID,
		Title:      fmt.Sprintf("Incident Resolved: %s", event.Data.Title),
		Body:       fmt.Sprintf("**Incident Resolved**\nTitle: %s\nSeverity: %s", event.Data.Title, event.Data.Severity),
	})
}

// mapAlertSeverity 将告警严重级别字符串映射为 P0-P3 优先级。
func mapAlertSeverity(severity string) string {
	switch severity {
	case "critical":
		return "P0"
	case "high":
		return "P1"
	case "medium":
		return "P2"
	case "low":
		return "P3"
	default:
		return "P3"
	}
}
