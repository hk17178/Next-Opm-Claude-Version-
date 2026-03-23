package service

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/event"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// AlertConsumer 处理来自告警服务的事件消息，将 alert.fired 事件转化为事件工单。
type AlertConsumer struct {
	uc     *biz.IncidentUsecase
	logger *zap.Logger
}

// NewAlertConsumer 创建一个新的告警消费者实例。
func NewAlertConsumer(uc *biz.IncidentUsecase, logger *zap.Logger) *AlertConsumer {
	return &AlertConsumer{uc: uc, logger: logger}
}

// alertFiredPayload 对应 alert.fired CloudEvent 的数据载荷结构。
type alertFiredPayload struct {
	AlertID   string `json:"alert_id"`
	RuleID    string `json:"rule_id"`
	RuleName  string `json:"rule_name"`
	Layer     int    `json:"layer"`
	Severity  string `json:"severity"`
	Source    struct {
		AssetID      string `json:"asset_id"`
		Hostname     string `json:"hostname"`
		IP           string `json:"ip"`
		AssetType    string `json:"asset_type"`
		BusinessUnit string `json:"business_unit"`
	} `json:"source"`
	Message      string            `json:"message"`
	MetricValue  float64           `json:"metric_value"`
	Threshold    float64           `json:"threshold"`
	Fingerprint  string            `json:"fingerprint"`
	Ironclad     bool              `json:"ironclad"`
	Tags         map[string]string `json:"tags"`
}

// HandleAlertFired 处理 alert.fired 事件，根据告警信息自动创建事件工单。
func (c *AlertConsumer) HandleAlertFired(ctx context.Context, evt *event.CloudEvent) error {
	var payload alertFiredPayload
	if err := json.Unmarshal(evt.Data, &payload); err != nil {
		c.logger.Error("failed to unmarshal alert.fired payload",
			zap.String("event_id", evt.ID),
			zap.Error(err),
		)
		return err
	}

	c.logger.Info("processing alert.fired event",
		zap.String("alert_id", payload.AlertID),
		zap.String("severity", payload.Severity),
		zap.String("hostname", payload.Source.Hostname),
	)

	// 从告警数据构建事件创建请求
	detectedAt := evt.Time
	req := biz.CreateIncidentRequest{
		Title:          payload.Source.Hostname + " " + payload.RuleName,
		Severity:       payload.Severity,
		SourceAlerts:   []string{payload.AlertID},
		AffectedAssets: []string{payload.Source.AssetID},
		BusinessUnit:   payload.Source.BusinessUnit,
		Tags:           payload.Tags,
		DetectedAt:     &detectedAt,
	}

	inc, err := c.uc.CreateIncident(ctx, req)
	if err != nil {
		c.logger.Error("failed to create incident from alert",
			zap.String("alert_id", payload.AlertID),
			zap.Error(err),
		)
		return err
	}

	c.logger.Info("incident created from alert",
		zap.String("incident_id", inc.IncidentID),
		zap.String("alert_id", payload.AlertID),
	)

	return nil
}

// HandleAIAnalysisDone 处理 ai.analysis.done 事件，将 AI 分析结果添加到事件时间线。
func (c *AlertConsumer) HandleAIAnalysisDone(ctx context.Context, evt *event.CloudEvent) error {
	var payload struct {
		AlertID           string   `json:"alert_id"`
		IncidentID        string   `json:"incident_id"`
		RootCause         string   `json:"root_cause"`
		Confidence        float64  `json:"confidence"`
		RootCauseCategory string   `json:"root_cause_category"`
		Evidence          []string `json:"evidence"`
		SuggestedActions  []string `json:"suggested_actions"`
		SimilarCases      []string `json:"similar_cases"`
		ModelUsed         string   `json:"model_used"`
	}
	if err := json.Unmarshal(evt.Data, &payload); err != nil {
		c.logger.Error("failed to unmarshal ai.analysis.done payload", zap.Error(err))
		return err
	}

	if payload.IncidentID == "" {
		c.logger.Warn("ai.analysis.done event missing incident_id, skipping")
		return nil
	}

	// 将 AI 分析结果作为 ai_analysis 类型条目添加到事件时间线
	entry := &biz.TimelineEntry{
		IncidentID: payload.IncidentID,
		Timestamp:  time.Now(),
		EntryType:  "ai_analysis",
		Source:     "ai",
		Content: map[string]any{
			"root_cause":          payload.RootCause,
			"confidence":          payload.Confidence,
			"root_cause_category": payload.RootCauseCategory,
			"evidence":            payload.Evidence,
			"suggested_actions":   payload.SuggestedActions,
			"similar_cases":       payload.SimilarCases,
			"model_used":          payload.ModelUsed,
		},
	}

	if err := c.uc.AddTimelineEntry(ctx, payload.IncidentID, entry); err != nil {
		c.logger.Error("failed to add AI analysis to timeline",
			zap.String("incident_id", payload.IncidentID),
			zap.Error(err),
		)
		return err
	}

	c.logger.Info("AI analysis added to incident timeline",
		zap.String("incident_id", payload.IncidentID),
		zap.Float64("confidence", payload.Confidence),
	)

	return nil
}
