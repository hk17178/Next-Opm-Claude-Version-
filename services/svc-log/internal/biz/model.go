// Package biz 定义日志服务的核心业务模型与领域逻辑，包括日志条目、采集源、解析规则、脱敏规则、保留策略等。
package biz

import (
	"time"
)

// LogEntry 表示存储在 Elasticsearch 中的一条日志记录。
// 字段命名与 OpenAPI 规范（svc-log.yaml）和 ES 索引映射（ON-003）保持一致。
type LogEntry struct {
	ID            string            `json:"id,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
	Level         LogLevel          `json:"level"`
	Message       string            `json:"message"`
	ServiceName   string            `json:"service_name,omitempty"`
	HostID        string            `json:"host_id,omitempty"`
	SourceType    string            `json:"source_type,omitempty"`
	SourceHost    string            `json:"source_host,omitempty"`
	SourceService string            `json:"source_service,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	TraceID       string            `json:"trace_id,omitempty"`
	SpanID        string            `json:"span_id,omitempty"`
	Extra         map[string]any    `json:"extra,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty"`
}

// LogLevel 枚举标准日志严重级别。
type LogLevel string

// 日志严重级别常量，从低到高依次为 DEBUG、INFO、WARN、ERROR、FATAL。
const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarn    LogLevel = "WARN"
	LogLevelError   LogLevel = "ERROR"
	LogLevelFatal   LogLevel = "FATAL"
	LogLevelUnknown LogLevel = "UNKNOWN"
)

// LogSource 定义日志采集源及其采集方式。
// 对应 ON-003 设计文档中的 log_sources 表。
type LogSource struct {
	SourceID         string    `json:"source_id"`
	Name             string    `json:"name"`
	SourceType       string    `json:"source_type"`        // host/middleware/network/security/transaction/database/k8s
	CollectionMethod string    `json:"collection_method"`  // syslog_tcp/syslog_udp/http_push/kafka_consume/webhook
	Config           any       `json:"config"`
	ParseRuleID      *string   `json:"parse_rule_id,omitempty"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ParseRule 描述日志解析规则，支持 json/syslog/grok/regex 等多种格式。
// 对应 ON-003 设计文档中的 parse_rules 表。
type ParseRule struct {
	RuleID       string `json:"rule_id"`
	Name         string `json:"name"`
	FormatType   string `json:"format_type"` // json/syslog/cef/clf/grok/regex/lua
	Pattern      string `json:"pattern,omitempty"`
	MultilineRule any   `json:"multiline_rule,omitempty"`
	FieldMapping any    `json:"field_mapping,omitempty"`
	SampleLog    string `json:"sample_log,omitempty"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

// MaskingRule 定义敏感数据脱敏规则，通过正则匹配并替换敏感内容。
// 对应 ON-003 设计文档中的 masking_rules 表。
type MaskingRule struct {
	RuleID      string `json:"rule_id"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Priority    int    `json:"priority"`
	Enabled     bool   `json:"enabled"`
}

// RetentionPolicy 定义日志保留策略的分层存储周期（热/温/冷）。
// 对应 ON-003 设计文档中的 retention_policies 表。
type RetentionPolicy struct {
	PolicyID   string `json:"policy_id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type,omitempty"`
	LogLevel   string `json:"log_level,omitempty"`
	HotDays    int    `json:"hot_days"`
	WarmDays   int    `json:"warm_days"`
	ColdDays   int    `json:"cold_days"`
	Enabled    bool   `json:"enabled"`
}

// Stream 表示一个日志流（带过滤条件和保留周期的视图）。
// 对应 OpenAPI 规范中的 /streams 资源。
type Stream struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Filter        string    `json:"filter,omitempty"`
	RetentionDays int       `json:"retention_days"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- API 请求/响应类型（与 OpenAPI svc-log.yaml 对齐） ---

// LogIngestRequest 是日志摄入 HTTP 请求体，遵循 OpenAPI 规范。
type LogIngestRequest struct {
	Entries []LogEntry `json:"entries"`
}

// LogIngestResponse 是日志摄入 HTTP 响应体，包含接受/拒绝计数。
type LogIngestResponse struct {
	Accepted int           `json:"accepted"`
	Rejected int           `json:"rejected"`
	Errors   []IngestError `json:"errors,omitempty"`
}

// IngestError 描述单条日志条目被拒绝的原因。
type IngestError struct {
	Index  int    `json:"index"`
	Reason string `json:"reason"`
}

// LogSearchRequest 是日志搜索请求，支持全文检索、时间范围和过滤条件。
type LogSearchRequest struct {
	Query     string            `json:"query"`
	TimeRange *TimeRange        `json:"time_range,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	PageSize  int               `json:"page_size,omitempty"`
	PageToken string            `json:"page_token,omitempty"`
	Sort      string            `json:"sort,omitempty"` // asc/desc
}

// TimeRange 表示搜索的时间窗口。
type TimeRange struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// LogSearchResponse 是日志搜索响应，包含匹配条目和分页令牌。
type LogSearchResponse struct {
	Total         int64      `json:"total"`
	Entries       []LogEntry `json:"entries"`
	NextPageToken string     `json:"next_page_token,omitempty"`
}

// LogStatsRequest 表示日志统计聚合查询请求。
type LogStatsRequest struct {
	GroupBy   string     `json:"group_by"` // source_type/level/time
	Interval  string    `json:"interval,omitempty"`
	TimeRange *TimeRange `json:"time_range,omitempty"`
}

// LogStatsResponse 包含聚合统计的桶数据。
type LogStatsResponse struct {
	Buckets []StatsBucket `json:"buckets"`
	Total   int64         `json:"total"`
}

// StatsBucket 是单个聚合统计桶。
type StatsBucket struct {
	Key      string  `json:"key"`
	DocCount int64   `json:"doc_count"`
	Value    float64 `json:"value,omitempty"`
}

// LogExportRequest 表示异步日志导出请求。
type LogExportRequest struct {
	Query     string            `json:"query"`
	TimeRange *TimeRange        `json:"time_range,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	Format    string            `json:"format"` // csv/json
}

// LogExportResponse 包含导出任务的状态信息。
type LogExportResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"` // pending/running/completed/failed
	URL    string `json:"url,omitempty"`
}

// IndexInfo 表示一个 Elasticsearch 索引的元信息，用于保留策略执行。
type IndexInfo struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Kafka CloudEvents（与 opsnexus.log.ingested.schema.json 对齐） ---

// CloudEvent 是 Kafka 事件的 CloudEvents v1.0 信封结构。
type CloudEvent struct {
	SpecVersion     string `json:"specversion"`
	ID              string `json:"id"`
	Type            string `json:"type"`
	Source          string `json:"source"`
	Time            string `json:"time"`
	DataContentType string `json:"datacontenttype,omitempty"`
	Data            any    `json:"data"`
}

// LogIngestedEventData 是 opsnexus.log.ingested 事件的数据载荷。
// 这是批次级摘要事件，而非逐条日志事件。
type LogIngestedEventData struct {
	BatchID       string         `json:"batch_id"`
	SourceType    string         `json:"source_type"`
	SourceHost    string         `json:"source_host"`
	SourceService string         `json:"source_service"`
	LogCount      int            `json:"log_count"`
	TimeRange     TimeRange      `json:"time_range"`
	LevelsSummary map[string]int `json:"levels_summary"`
	ESIndex       string         `json:"es_index"`
	ParseRuleUsed string         `json:"parse_rule_used,omitempty"`
}

// StreamCreateRequest 是创建日志流的请求体。
type StreamCreateRequest struct {
	Name          string `json:"name"`
	Filter        string `json:"filter,omitempty"`
	RetentionDays int    `json:"retention_days,omitempty"`
}

// StreamListResponse 是日志流列表响应，支持分页。
type StreamListResponse struct {
	Streams       []*Stream `json:"streams"`
	NextPageToken string    `json:"next_page_token,omitempty"`
}

// ErrorResponse 是通用错误响应结构，遵循 OpenAPI 规范。
type ErrorResponse struct {
	Error string `json:"error"`
}
