// Package biz 实现告警服务的核心业务逻辑，包括 6 层告警评估引擎、规则管理、
// 基线检测、去重抑制等功能。
package biz

import (
	"encoding/json"
	"time"
)

// Severity 表示告警严重等级，与 OpenAPI 规范对齐（critical/high/medium/low/info），
// 内部优先级映射为 P0=critical, P1=high, P2=medium, P3=low, P4=info。
type Severity string

const (
	SeverityCritical Severity = "critical" // P0
	SeverityHigh     Severity = "high"     // P1
	SeverityMedium   Severity = "medium"   // P2
	SeverityLow      Severity = "low"      // P3
	SeverityInfo     Severity = "info"     // P4
)

// AlertStatus 表示告警实例的生命周期状态：触发中(firing)、已确认(acknowledged)、已解决(resolved)。
type AlertStatus string

const (
	AlertStatusFiring       AlertStatus = "firing"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
)

// RuleType 表示告警规则类型，共 7 种，分布在 6 层评估引擎（Layer 0-5）中。
type RuleType string

const (
	RuleTypeThreshold RuleType = "threshold" // Layer 1
	RuleTypeKeyword   RuleType = "keyword"   // Layer 1
	RuleTypeFrequency RuleType = "frequency" // Layer 1
	RuleTypeBaseline  RuleType = "baseline"  // Layer 2
	RuleTypeTrend     RuleType = "trend"     // Layer 3
	RuleTypeAI        RuleType = "ai"        // Layer 4
	RuleTypeBusiness  RuleType = "business"  // Layer 5
)

// AlertRule 定义单条告警规则，对应 ON-003 数据库设计中的 alert_rules 表。
// 包含规则的层级、类型、触发条件、严重等级等核心属性。
type AlertRule struct {
	RuleID          string          `json:"id" db:"rule_id"`
	Name            string          `json:"name" db:"name"`
	Description     string          `json:"description" db:"description"`
	Layer           int             `json:"layer" db:"layer"`                         // 0-5
	RuleType        RuleType        `json:"rule_type" db:"rule_type"`                 // threshold/keyword/frequency/baseline/trend/ai/business
	Condition       json.RawMessage `json:"condition" db:"condition"`                 // JSONB rule expression
	Targets         json.RawMessage `json:"targets,omitempty" db:"targets"`           // JSONB asset/asset-group refs
	Severity        Severity        `json:"severity" db:"severity"`
	Ironclad        bool            `json:"ironclad" db:"ironclad"`                   // Layer 0 iron-law flag
	Enabled         bool            `json:"enabled" db:"enabled"`
	Schedule        json.RawMessage `json:"schedule,omitempty" db:"schedule"`         // JSONB timed activation
	CooldownMinutes int             `json:"cooldown_minutes" db:"cooldown_minutes"`
	Labels          map[string]string `json:"labels,omitempty"`
	NotificationCh  []string        `json:"notification_channels,omitempty"`
	CreatedBy       string          `json:"created_by,omitempty" db:"created_by"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

// Alert 表示一条已触发的告警实例，对应 ON-003 中的 alerts 表。
// 记录告警的来源、严重等级、状态以及触发时的指标值等上下文信息。
type Alert struct {
	AlertID        string            `json:"id" db:"alert_id"`              // ALT-YYYYMMDD-NNN
	RuleID         string            `json:"rule_id" db:"rule_id"`
	Severity       Severity          `json:"severity" db:"severity"`
	Status         AlertStatus       `json:"status" db:"status"`            // firing/acknowledged/resolved
	Title          string            `json:"title" db:"title"`
	Description    string            `json:"description,omitempty" db:"description"`
	SourceHost     string            `json:"source_host,omitempty" db:"source_host"`
	SourceService  string            `json:"source_service,omitempty" db:"source_service"`
	SourceAssetID  string            `json:"source_asset_id,omitempty" db:"source_asset_id"`
	Message        string            `json:"message" db:"message"`
	MetricValue    *float64          `json:"metric_value,omitempty" db:"metric_value"`
	ThresholdValue *float64          `json:"threshold_value,omitempty" db:"threshold_value"`
	Fingerprint    string            `json:"fingerprint" db:"fingerprint"`
	Layer          int               `json:"layer" db:"layer"`
	Ironclad       bool              `json:"ironclad" db:"ironclad"`
	Suppressed     bool              `json:"suppressed" db:"suppressed"`
	SuppressedBy   string            `json:"suppressed_by,omitempty" db:"suppressed_by"`
	TriggeredAt    time.Time         `json:"fired_at" db:"triggered_at"`
	AcknowledgedAt *time.Time        `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty" db:"resolved_at"`
	IncidentID     string            `json:"incident_id,omitempty" db:"incident_id"`
	Tags           map[string]string `json:"labels,omitempty" db:"tags"`
}

// BaselineModel 存储已学习的指标基线模型，对应 ON-003 中的 baseline_models 表。
// 用于 Layer 2 动态基线异常检测，记录采样间隔、学习天数和偏差百分比等参数。
type BaselineModel struct {
	ModelID          string          `json:"model_id" db:"model_id"`
	RuleID           string          `json:"rule_id" db:"rule_id"`
	MetricName       string          `json:"metric_name" db:"metric_name"`
	Target           string          `json:"target" db:"target"`
	SamplingInterval string          `json:"sampling_interval" db:"sampling_interval"` // 1min/5min/15min/30min/1h
	LearningDays     int             `json:"learning_days" db:"learning_days"`
	DeviationPct     float64         `json:"deviation_pct" db:"deviation_pct"`
	PeakExemptions   json.RawMessage `json:"peak_exemptions,omitempty" db:"peak_exemptions"`
	BaselineData     json.RawMessage `json:"baseline_data,omitempty" db:"baseline_data"`
	Status           string          `json:"status" db:"status"` // learning/active/paused
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	LastTrainedAt    *time.Time      `json:"last_trained_at,omitempty" db:"last_trained_at"`
}

// Silence 表示告警静默/抑制规则，对应 OpenAPI silences 接口。
// 在指定时间范围内，匹配标签的告警将被静默处理。
type Silence struct {
	ID        string           `json:"id"`
	Matchers  []SilenceMatcher `json:"matchers"`
	StartsAt  time.Time        `json:"starts_at"`
	EndsAt    time.Time        `json:"ends_at"`
	Comment   string           `json:"comment,omitempty"`
	CreatedBy string           `json:"created_by,omitempty"`
}

// SilenceMatcher 定义静默规则的标签匹配条件，支持精确匹配和正则匹配。
type SilenceMatcher struct {
	Label   string `json:"label"`
	Value   string `json:"value"`
	IsRegex bool   `json:"is_regex"`
}

// MetricSample 表示从 Kafka 接收的指标数据点，是告警引擎的输入之一。
type MetricSample struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels"`
	Value      float64           `json:"value"`
	Timestamp  time.Time         `json:"timestamp"`
	HostID     string            `json:"host_id,omitempty"`
	Service    string            `json:"service_name,omitempty"`
}

// LogEvent 表示从 Kafka 接收的日志事件，是告警引擎的输入之一。
type LogEvent struct {
	Source    string            `json:"source"`
	Level    string            `json:"level"`
	Message  string            `json:"message"`
	Labels   map[string]string `json:"labels"`
	HostID   string            `json:"host_id,omitempty"`
	Service  string            `json:"service_name,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
}

// ThresholdCondition 定义阈值规则的 JSONB 条件结构（Layer 1），
// 当指标值与阈值按指定操作符比较为真时触发告警。
type ThresholdCondition struct {
	MetricName string  `json:"metric_name"`
	Operator   string  `json:"operator"` // >, <, =, >=, <=, !=
	Threshold  float64 `json:"threshold"`
	ForMinutes int     `json:"for_minutes"`
}

// KeywordCondition 定义关键字/正则规则的 JSONB 条件结构（Layer 1），
// 当日志消息匹配指定正则模式时触发告警。
type KeywordCondition struct {
	Pattern string `json:"pattern"`
}

// FrequencyCondition 定义频率规则的 JSONB 条件结构（Layer 1），
// 当指定事件在滑动时间窗口内出现次数达到阈值时触发告警。
type FrequencyCondition struct {
	Event         string `json:"event"`
	Count         int    `json:"count"`
	WindowMinutes int    `json:"window_minutes"`
}

// TrendCondition 定义趋势规则的 JSONB 条件结构（Layer 3），
// 通过同比（日/周/月）对比检测指标的变化趋势，超过阈值时触发告警。
type TrendCondition struct {
	MetricName      string  `json:"metric_name"`
	CompareWindow   string  `json:"compare_window"`   // "1d" | "1w" | "1m"
	ChangeThreshold float64 `json:"change_threshold"` // e.g. 20.0 = 20% increase
	Direction       string  `json:"direction"`        // "up" | "down" | "either"
}

// --- 仓储接口 ---

// RuleRepository 定义告警规则的持久化操作接口。
type RuleRepository interface {
	Create(rule *AlertRule) error
	Update(rule *AlertRule) error
	Delete(id string) error
	GetByID(id string) (*AlertRule, error)
	List(enabled *bool, pageSize int, pageToken string) ([]*AlertRule, string, error)
	ListByLayerAndType(layer int, ruleType RuleType) ([]*AlertRule, error)
	ListEnabled() ([]*AlertRule, error)
}

// AlertRepository 定义告警实例的持久化操作接口。
type AlertRepository interface {
	Create(alert *Alert) error
	GetByID(id string) (*Alert, error)
	UpdateStatus(id string, status AlertStatus, acknowledgedAt, resolvedAt *time.Time) error
	GetByFingerprint(fingerprint string) (*Alert, error)
	GetActiveByRuleID(ruleID string) ([]*Alert, error)
	List(status *AlertStatus, severity *Severity, pageSize int, pageToken string) ([]*Alert, string, error)
	IncrementSuppression(id string, suppressedBy string) error
	NextAlertID() (string, error)
}

// SilenceRepository 定义告警静默规则的持久化操作接口。
type SilenceRepository interface {
	Create(s *Silence) error
	List() ([]*Silence, error)
	GetActive(labels map[string]string) ([]*Silence, error)
}
