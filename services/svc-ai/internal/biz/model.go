// Package biz 包含 svc-ai 服务的核心业务逻辑，包括 AI 分析任务编排、
// 模型管理、上下文收集、数据脱敏和熔断保护等功能。
package biz

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AnalysisType 表示 AI 分析的类型，对应不同的分析场景。
type AnalysisType string

// AI 分析类型常量，每种类型映射到不同的 prompt 场景（scene）。
const (
	AnalysisRootCause       AnalysisType = "root_cause"        // 根因分析
	AnalysisAnomalyDetect   AnalysisType = "anomaly_detection"  // 异常检测
	AnalysisPrediction      AnalysisType = "prediction"          // 趋势预测
	AnalysisCorrelation     AnalysisType = "correlation"         // 关联分析
	AnalysisLogSummary      AnalysisType = "log_summary"         // 日志摘要
)

// AnalysisStatus 表示分析任务的生命周期状态。
type AnalysisStatus string

// 分析任务状态常量，遵循 pending → running → success/partial/failed 的流转。
const (
	StatusPending AnalysisStatus = "pending" // 已创建，等待执行
	StatusRunning AnalysisStatus = "running" // 正在执行 AI 模型调用
	StatusSuccess AnalysisStatus = "success" // 分析完成，结果已解析
	StatusPartial AnalysisStatus = "partial" // 模型返回了结果但解析不完整
	StatusFailed  AnalysisStatus = "failed"  // 分析失败（熔断、模型错误等）
)

// RootCauseCategory 是五类根因分类法，用于对 AI 分析出的根因进行归类。
type RootCauseCategory string

// 根因分类常量，覆盖人为操作、变更引发、系统故障、外部因素和未知原因五大类。
const (
	CauseHumanAction  RootCauseCategory = "human_action"  // 人为操作引起
	CauseChangeCaused RootCauseCategory = "change_caused"  // 变更引发
	CauseSystemFault  RootCauseCategory = "system_fault"   // 系统自身故障
	CauseExternal     RootCauseCategory = "external"       // 外部因素（如供应商、网络）
	CauseUnknown      RootCauseCategory = "unknown"        // 无法确定根因
)

// AnalysisTask 是 AI 分析任务的核心领域实体。
// 每个任务由事件触发（如 alert.fired 或 incident.created），异步执行分析流程，
// 最终产出结构化的 AnalysisResult。
type AnalysisTask struct {
	ID             uuid.UUID       `json:"id"`
	Type           AnalysisType    `json:"type"`
	Status         AnalysisStatus  `json:"status"`
	IncidentID     *uuid.UUID      `json:"incident_id,omitempty"`
	AlertIDs       []uuid.UUID     `json:"alert_ids,omitempty"`
	TimeRange      *TimeRange      `json:"time_range,omitempty"`
	Context        json.RawMessage `json:"context,omitempty"`
	Result         *AnalysisResult `json:"result,omitempty"`
	ModelVersion   string          `json:"model_version,omitempty"`
	TriggerEventID string          `json:"trigger_event_id,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
}

// TimeRange 表示时间范围，用于限定分析任务的数据查询窗口。
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// AnalysisResult 保存 AI 模型的结构化输出结果，包括摘要、置信度、根因列表和建议操作。
type AnalysisResult struct {
	Summary         string           `json:"summary"`
	Confidence      float64          `json:"confidence"`
	RootCauses      []RootCause      `json:"root_causes,omitempty"`
	Recommendations []string         `json:"recommendations,omitempty"`
	Anomalies       []Anomaly        `json:"anomalies,omitempty"`
	RawOutput       json.RawMessage  `json:"raw_output,omitempty"`
}

// RootCause 表示一个根因分析结果条目，包含描述、概率、分类和证据链。
type RootCause struct {
	Description   string   `json:"description"`
	Probability   float64  `json:"probability"`
	Category      RootCauseCategory `json:"category,omitempty"`
	RelatedCIIDs  []string `json:"related_ci_ids,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

// Anomaly 表示一个异常检测结果，记录指标的预期值与实际值偏差。
type Anomaly struct {
	Metric        string    `json:"metric"`
	Timestamp     time.Time `json:"timestamp"`
	ExpectedValue float64   `json:"expected_value"`
	ActualValue   float64   `json:"actual_value"`
	Severity      string    `json:"severity"`
}

// AIModel 表示已配置的 AI 模型（云端或本地部署），包含连接信息、速率限制和健康状态。
type AIModel struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Provider        string          `json:"provider"`
	DeploymentType  string          `json:"deployment_type"`
	APIEndpoint     string          `json:"api_endpoint"`
	APIKeyEncrypted []byte          `json:"-"`
	LocalEndpoint   string          `json:"local_endpoint,omitempty"`
	LocalModelName  string          `json:"local_model_name,omitempty"`
	Parameters      json.RawMessage `json:"parameters,omitempty"`
	RateLimitQPS    int             `json:"rate_limit_qps"`
	Enabled         bool            `json:"enabled"`
	HealthStatus    string          `json:"health_status"`
	LastHealthCheck *time.Time      `json:"last_health_check,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// SceneBinding 将 AI 场景映射到主模型/备用模型及路由策略。
// 路由策略支持 cloud_first（优先云端）、local_first（优先本地）等。
type SceneBinding struct {
	ID              uuid.UUID  `json:"id"`
	Scene           string     `json:"scene"`
	PrimaryModelID  *uuid.UUID `json:"primary_model_id,omitempty"`
	FallbackModelID *uuid.UUID `json:"fallback_model_id,omitempty"`
	RoutingStrategy string     `json:"routing_strategy"`
	PromptVersion   string     `json:"prompt_version,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Prompt 是场景化的版本化提示词模板，支持变量替换和 A/B 测试。
type Prompt struct {
	ID            uuid.UUID       `json:"id"`
	Scene         string          `json:"scene"`
	Version       string          `json:"version"`
	SystemPrompt  string          `json:"system_prompt"`
	UserPrompt    string          `json:"user_prompt"`
	Variables     json.RawMessage `json:"variables,omitempty"`
	IsActive      bool            `json:"is_active"`
	FeedbackScore *float64        `json:"feedback_score,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// AICallLog 记录每次 AI 模型调用的详细信息，包括 token 消耗、延迟和状态，用于审计和成本分析。
type AICallLog struct {
	ID            uuid.UUID  `json:"id"`
	AnalysisID    *uuid.UUID `json:"analysis_id,omitempty"`
	ModelID       *uuid.UUID `json:"model_id,omitempty"`
	Scene         string     `json:"scene"`
	PromptVersion string     `json:"prompt_version,omitempty"`
	InputTokens   int        `json:"input_tokens"`
	OutputTokens  int        `json:"output_tokens"`
	LatencyMs     int        `json:"latency_ms"`
	Status        string     `json:"status"`
	Feedback      string     `json:"feedback,omitempty"`
	InputHash     string     `json:"input_hash,omitempty"`
	OutputSummary string     `json:"output_summary,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AIBudget 按月按模型跟踪 token 用量和预算，超过 80% 触发告警，100% 触发熔断切换备用模型。
type AIBudget struct {
	Month       string    `json:"month"`
	ModelID     uuid.UUID `json:"model_id"`
	TokensUsed  int64     `json:"tokens_used"`
	BudgetLimit int64     `json:"budget_limit"`
	AlertSent   bool      `json:"alert_sent"`
	Exhausted   bool      `json:"exhausted"`
}

// KnowledgeEntry 表示知识库条目，用于为 AI 分析提供历史案例和上下文参考。
type KnowledgeEntry struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatRequest 是所有模型适配器的统一请求结构，屏蔽不同厂商 API 的差异。
type ChatRequest struct {
	SystemPrompt string  `json:"system_prompt"`
	UserPrompt   string  `json:"user_prompt"`
	Temperature  float64 `json:"temperature,omitempty"`
	MaxTokens    int     `json:"max_tokens,omitempty"`
}

// ChatResponse 是模型适配器的统一响应结构，包含生成内容和 token 统计。
type ChatResponse struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	ModelName    string `json:"model_name"`
}

// AnalysisRepo 定义分析任务的持久化接口。
type AnalysisRepo interface {
	Create(ctx context.Context, task *AnalysisTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*AnalysisTask, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status AnalysisStatus, result *AnalysisResult, errMsg string) error
	List(ctx context.Context, filter AnalysisFilter) ([]*AnalysisTask, string, error)
	SaveFeedback(ctx context.Context, analysisID uuid.UUID, req FeedbackRequest) error
}

// AnalysisFilter 定义分析任务列表查询的过滤条件。
type AnalysisFilter struct {
	Status    *AnalysisStatus
	Type      *AnalysisType
	PageToken string
	PageSize  int
}

// ModelRepo 定义 AI 模型配置的数据访问接口。
type ModelRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*AIModel, error)
	List(ctx context.Context) ([]*AIModel, error)
	GetSceneBinding(ctx context.Context, scene string) (*SceneBinding, error)
	UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string) error
}

// PromptRepo 定义提示词模板的数据访问接口。
type PromptRepo interface {
	GetActive(ctx context.Context, scene string) (*Prompt, error)
	GetByVersion(ctx context.Context, scene, version string) (*Prompt, error)
	List(ctx context.Context) ([]*Prompt, error)
	Create(ctx context.Context, p *Prompt) error
}

// CallLogRepo 定义 AI 调用日志的记录接口。
type CallLogRepo interface {
	Create(ctx context.Context, log *AICallLog) error
}

// BudgetRepo 定义 token 预算的跟踪接口。
type BudgetRepo interface {
	GetOrCreate(ctx context.Context, month string, modelID uuid.UUID, defaultLimit int64) (*AIBudget, error)
	IncrementUsage(ctx context.Context, month string, modelID uuid.UUID, tokens int64) error
}

// KnowledgeRepo 定义知识库的数据访问接口，支持语义搜索。
type KnowledgeRepo interface {
	Create(ctx context.Context, entry *KnowledgeEntry) error
	GetByID(ctx context.Context, id uuid.UUID) (*KnowledgeEntry, error)
	List(ctx context.Context, pageToken string, pageSize int) ([]*KnowledgeEntry, string, error)
	Search(ctx context.Context, query string, topK int) ([]*KnowledgeEntry, []float64, error)
}
