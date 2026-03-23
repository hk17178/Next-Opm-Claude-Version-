// Package biz 定义通知服务的核心业务模型与领域逻辑，包括通知渠道、机器人、广播规则和去重引擎。
package biz

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChannelType 枚举支持的通知渠道类型。
type ChannelType string

const (
	ChannelWecomWebhook ChannelType = "wecom_webhook"
	ChannelWecomApp     ChannelType = "wecom_app"
	ChannelSMS          ChannelType = "sms"
	ChannelVoice        ChannelType = "phone"
	ChannelEmail        ChannelType = "email"
	ChannelWebhook      ChannelType = "webhook"
	ChannelFeishu       ChannelType = "feishu"
	ChannelDingtalk     ChannelType = "dingtalk"
)

// NotifyStatus 表示通知投递状态。
type NotifyStatus string

const (
	StatusSent       NotifyStatus = "sent"
	StatusFailed     NotifyStatus = "failed"
	StatusMerged     NotifyStatus = "merged"
	StatusSuppressed NotifyStatus = "suppressed"
)

// LifecycleNode 是事件生命周期中的 7 个广播节点之一。
type LifecycleNode string

const (
	NodeAlertFired          LifecycleNode = "alert_fired"
	NodeIncidentCreated     LifecycleNode = "incident_created"
	NodeAIAnalysisDone      LifecycleNode = "ai_analysis_done"
	NodeIncidentAcknowledged LifecycleNode = "incident_acknowledged"
	NodePhaseChanged        LifecycleNode = "incident_phase_changed"
	NodeIncidentResolved    LifecycleNode = "incident_resolved"
	NodePostmortem          LifecycleNode = "incident_postmortem"
)

// Bot 表示一个已配置的通知渠道实例（机器人），包含渠道配置、健康状态和作用域信息。
type Bot struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	ChannelType     ChannelType     `json:"channel_type"`
	Config          json.RawMessage `json:"config"`
	Scope           json.RawMessage `json:"scope,omitempty"`
	TemplateType    string          `json:"template_type,omitempty"`
	HealthStatus    string          `json:"health_status"`
	LastHealthCheck *time.Time      `json:"last_health_check,omitempty"`
	FailureCount    int             `json:"failure_count"`
	Enabled         bool            `json:"enabled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// --- 各渠道类型的配置结构 ---

// WecomWebhookConfig 企业微信群机器人 Webhook 配置。
type WecomWebhookConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// WecomAppConfig 企业微信应用消息配置。
type WecomAppConfig struct {
	CorpID     string `json:"corp_id"`
	AgentID    int    `json:"agent_id"`
	Secret     string `json:"secret"`
	ToUser     string `json:"to_user,omitempty"`
	ToParty    string `json:"to_party,omitempty"`
}

// SMSConfig 短信通知渠道配置，支持阿里云/腾讯云等主流服务商。
type SMSConfig struct {
	Provider    string `json:"provider"`               // 短信服务商名称（aliyun/tencent 等）
	AccessKey   string `json:"access_key"`             // 服务商 API 访问密钥 ID
	Secret      string `json:"secret"`                 // 服务商 API 密钥，用于 HMAC 签名
	SignName    string `json:"sign_name"`              // 短信签名（如"OpsNexus"，需在服务商后台审核通过）
	Template    string `json:"template_code"`           // 短信模板编号（如 SMS_12345）
	APIEndpoint string `json:"api_endpoint,omitempty"` // 自定义 API 端点（可选，留空则使用服务商默认端点）
}

// EmailConfig 邮件通知渠道（SMTP）配置。
type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	UseTLS   bool   `json:"use_tls"`
}

// WebhookConfig 通用 Webhook 通知渠道配置，支持 HMAC 签名验证。
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Secret  string            `json:"secret,omitempty"`
}

// VoiceConfig 语音 TTS 通知渠道配置，支持阿里云/腾讯云等主流服务商。
type VoiceConfig struct {
	Provider    string `json:"provider"`               // 语音服务商名称（aliyun/tencent 等）
	AccessKey   string `json:"access_key"`             // 服务商 API 访问密钥 ID
	Secret      string `json:"secret"`                 // 服务商 API 密钥，用于 HMAC 签名
	Template    string `json:"template_code"`           // 语音模板编号
	CallerID    string `json:"caller_id"`              // 主叫号码（外呼显示号码）
	APIEndpoint string `json:"api_endpoint,omitempty"` // 自定义 API 端点（可选，留空则使用服务商默认端点）
}

// FeishuConfig 飞书群机器人 Webhook 配置。
type FeishuConfig struct {
	WebhookURL string `json:"webhook_url"` // 飞书自定义机器人 Webhook 地址
}

// DingtalkConfig 钉钉群机器人 Webhook 配置，支持可选的 HMAC-SHA256 签名验证。
type DingtalkConfig struct {
	WebhookURL string `json:"webhook_url"`          // 钉钉自定义机器人 Webhook 地址
	Secret     string `json:"secret,omitempty"`      // 签名密钥（可选，配置后启用 HMAC-SHA256 签名验证）
	DetailURL  string `json:"detail_url,omitempty"`  // "查看详情"按钮跳转链接（可选）
}

// NotificationLog 记录每次通知发送的详细日志，包括状态、错误信息和重试次数。
type NotificationLog struct {
	ID             uuid.UUID    `json:"id"`
	BotID          *uuid.UUID   `json:"bot_id,omitempty"`
	ChannelType    ChannelType  `json:"channel_type"`
	Recipient      string       `json:"recipient,omitempty"`
	MessageType    string       `json:"message_type,omitempty"`
	EventNode      string       `json:"event_node,omitempty"`
	IncidentID     *uuid.UUID   `json:"incident_id,omitempty"`
	AlertID        *uuid.UUID   `json:"alert_id,omitempty"`
	ContentHash    string       `json:"content_hash,omitempty"`
	ContentPreview string       `json:"content_preview,omitempty"`
	Status         NotifyStatus `json:"status"`
	ErrorMessage   string       `json:"error_message,omitempty"`
	RetryCount     int          `json:"retry_count"`
	SentAt         time.Time    `json:"sent_at"`
}

// BroadcastRule 配置机器人在特定生命周期节点触发广播的规则，支持严重级别过滤和自定义模板。
type BroadcastRule struct {
	ID             uuid.UUID       `json:"id"`
	BotID          uuid.UUID       `json:"bot_id"`
	EventNode      string          `json:"event_node"`
	SeverityFilter json.RawMessage `json:"severity_filter,omitempty"`
	Frequency      json.RawMessage `json:"frequency,omitempty"`
	Template       string          `json:"template,omitempty"`
	Enabled        bool            `json:"enabled"`
	CreatedAt      time.Time       `json:"created_at"`
}

// SendRequest 是 API 层发送通知的请求结构。
type SendRequest struct {
	Channel    ChannelType    `json:"channel"`
	Recipients []string       `json:"recipients"`
	TemplateID string         `json:"template_id,omitempty"`
	Content    MessageContent `json:"content"`
	Priority   string         `json:"priority,omitempty"`
	IncidentID *uuid.UUID     `json:"incident_id,omitempty"`
	AlertID    *uuid.UUID     `json:"alert_id,omitempty"`
}

// MessageContent 是通知消息的内容结构，支持主题、正文和模板变量。
type MessageContent struct {
	Subject   string            `json:"subject,omitempty"`
	Body      string            `json:"body"`
	Variables map[string]string `json:"variables,omitempty"`
}

// BroadcastEvent 是生命周期广播的内部事件结构。
type BroadcastEvent struct {
	Node       LifecycleNode `json:"node"`
	Severity   string        `json:"severity"`
	IncidentID *uuid.UUID    `json:"incident_id,omitempty"`
	AlertID    *uuid.UUID    `json:"alert_id,omitempty"`
	Title      string        `json:"title"`
	Body       string        `json:"body"`
	ExtraVars  map[string]string `json:"extra_vars,omitempty"`
}

// --- 仓储接口 ---

// BotRepo 定义通知机器人的数据访问接口。
type BotRepo interface {
	Create(ctx context.Context, bot *Bot) error
	GetByID(ctx context.Context, id uuid.UUID) (*Bot, error)
	List(ctx context.Context) ([]*Bot, error)
	Update(ctx context.Context, bot *Bot) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string, failureCount int) error
	ListEnabled(ctx context.Context) ([]*Bot, error)
}

// NotificationLogRepo 定义通知日志的数据访问接口。
type NotificationLogRepo interface {
	Create(ctx context.Context, log *NotificationLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*NotificationLog, error)
	List(ctx context.Context, filter NotificationFilter) ([]*NotificationLog, string, error)
}

// NotificationFilter 是通知日志查询的过滤条件。
type NotificationFilter struct {
	Channel   *ChannelType
	Status    *NotifyStatus
	PageToken string
	PageSize  int
}

// BroadcastRuleRepo 定义广播规则的数据访问接口。
type BroadcastRuleRepo interface {
	Create(ctx context.Context, rule *BroadcastRule) error
	ListByNode(ctx context.Context, node string) ([]*BroadcastRule, error)
	List(ctx context.Context) ([]*BroadcastRule, error)
}
