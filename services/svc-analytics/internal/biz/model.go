// Package biz 包含 svc-analytics 服务的核心业务逻辑，涵盖 SLA 计算、
// 指标查询、误差预算管理、知识库、仪表盘和报表等功能。
package biz

import (
	"time"
)

// SLA 计算维度层级（FR-10-003：支持任意维度组合）。
// 每个维度对应一种 SLA 统计粒度，可按需组合。
const (
	SLALevelInfraLayer   = "infra_layer"    // network/host/application/database/middleware
	SLALevelBusinessUnit = "business_unit"
	SLALevelAssetGroup   = "asset_group"
	SLALevelAssetGrade   = "asset_grade"
	SLALevelRegion       = "region"
	SLALevelAsset        = "asset"
	SLALevelGlobal       = "global"
)

// DefaultSLATargets 将资产等级映射到默认 SLA 目标值（FR-10-006）。
// S 级最高（99.99%），D 级最低（99.00%），体现资产重要性分级保护策略。
var DefaultSLATargets = map[string]float64{
	"S": 99.99,
	"A": 99.95,
	"B": 99.90,
	"C": 99.50,
	"D": 99.00,
}

// 报表生成状态常量。
const (
	ReportStatusPending   = "pending"
	ReportStatusRunning   = "running"
	ReportStatusCompleted = "completed"
	ReportStatusFailed    = "failed"
)

// 报表输出格式常量（对应 OpenAPI 规范中支持的格式）。
const (
	ReportFormatJSON = "json"
	ReportFormatCSV  = "csv"
	ReportFormatPDF  = "pdf"
)

// 知识库文章类型常量（FR-17-002）。
const (
	KnowledgeTypeCaseStudy = "case_study"   // 故障案例
	KnowledgeTypeRunbook   = "runbook"      // Runbook
	KnowledgeTypeFAQ       = "faq"          // FAQ
	KnowledgeTypeArchDoc   = "architecture" // 架构文档
	KnowledgeTypeVendorDoc = "vendor_doc"   // 厂商手册
)

// 知识库文章状态常量。
const (
	KnowledgeStatusDraft     = "draft"
	KnowledgeStatusPublished = "published"
	KnowledgeStatusArchived  = "archived"
)

// 仪表盘面板类型常量（对应 OpenAPI 规范）。
const (
	PanelTypeLineChart = "line_chart"
	PanelTypeBarChart  = "bar_chart"
	PanelTypePieChart  = "pie_chart"
	PanelTypeTable     = "table"
	PanelTypeStat      = "stat"
	PanelTypeHeatmap   = "heatmap"
)

// SLAConfig 定义特定范围的 SLA 目标配置，包含维度、目标百分比和统计窗口。
type SLAConfig struct {
	ConfigID         string    `json:"config_id" db:"config_id"`
	Name             string    `json:"name" db:"name"`
	Dimension        string    `json:"dimension" db:"dimension"`                 // infra_layer/business_unit/asset_group/asset_grade/region/asset/global
	DimensionValue   string    `json:"dimension_value" db:"dimension_value"`     // e.g., "payment", "S", "cn-east"
	TargetPercentage float64   `json:"target_percentage" db:"target_percentage"` // e.g., 99.95
	Window           string    `json:"window" db:"window"`                       // monthly/quarterly/yearly/weekly
	ExcludePlanned   bool      `json:"exclude_planned" db:"exclude_planned"`     // FR-10-002: exclude planned maintenance
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// SLAResult 保存指定周期内的 SLA 计算结果。
// FR-10-001 计算公式：SLA% = (总时间 - 非计划停机时间) / 总时间 * 100%
// FR-10-004 误差预算：错误预算 = 总时间 * (1 - 目标/100)，剩余百分比 = (预算 - 已用) / 预算 * 100%
type SLAResult struct {
	ConfigID           string    `json:"config_id"`
	Dimension          string    `json:"dimension"`
	DimensionValue     string    `json:"dimension_value"`
	TargetPercentage   float64   `json:"target_percentage"`
	ActualPercentage   float64   `json:"actual_percentage"`
	Compliance         bool      `json:"compliance"`
	PeriodStart        time.Time `json:"period_start"`
	PeriodEnd          time.Time `json:"period_end"`
	TotalSeconds       float64   `json:"total_seconds"`
	DowntimeSeconds    float64   `json:"downtime_seconds"`
	ErrorBudgetTotal   float64   `json:"error_budget_total_seconds"`
	ErrorBudgetUsed    float64   `json:"error_budget_used_seconds"`
	ErrorBudgetRemain  float64   `json:"error_budget_remaining_pct"` // FR-10-004
	IncidentCount      int       `json:"incident_count"`
}

// SLAIncident 表示 ClickHouse sla_incidents 表的一行数据（从事件领域 ETL 同步）。
type SLAIncident struct {
	IncidentID        string    `json:"incident_id" ch:"incident_id"`
	Severity          string    `json:"severity" ch:"severity"`
	BusinessUnit      string    `json:"business_unit" ch:"business_unit"`
	InfraLayer        string    `json:"infra_layer" ch:"infra_layer"`
	AssetGroup        string    `json:"asset_group" ch:"asset_group"`
	AssetGrade        string    `json:"asset_grade" ch:"asset_grade"`
	Region            string    `json:"region" ch:"region"`
	DetectedAt        time.Time `json:"detected_at" ch:"detected_at"`
	ResolvedAt        time.Time `json:"resolved_at" ch:"resolved_at"`
	DowntimeSeconds   uint32    `json:"downtime_seconds" ch:"downtime_seconds"`
	IsPlanned         uint8     `json:"is_planned" ch:"is_planned"`
	RootCauseCategory string    `json:"root_cause_category" ch:"root_cause_category"`
}

// BusinessMetric 表示 ClickHouse business_metrics 表的一行业务指标数据。
type BusinessMetric struct {
	Timestamp    time.Time         `json:"timestamp" ch:"timestamp"`
	MetricName   string            `json:"metric_name" ch:"metric_name"`
	MetricValue  float64           `json:"metric_value" ch:"metric_value"`
	BusinessUnit string            `json:"business_unit" ch:"business_unit"`
	Service      string            `json:"service" ch:"service"`
	Tags         map[string]string `json:"tags,omitempty" ch:"tags"`
}

// ResourceMetric 表示 ClickHouse resource_metrics 表的一行资源指标数据。
type ResourceMetric struct {
	Timestamp    time.Time `json:"timestamp" ch:"timestamp"`
	MetricName   string    `json:"metric_name" ch:"metric_name"`
	MetricValue  float64   `json:"metric_value" ch:"metric_value"`
	AssetID      string    `json:"asset_id" ch:"asset_id"`
	AssetType    string    `json:"asset_type" ch:"asset_type"`
	BusinessUnit string    `json:"business_unit" ch:"business_unit"`
	Region       string    `json:"region" ch:"region"`
}

// MetricIngestRequest 对应 OpenAPI MetricIngestRequest 结构，用于批量写入指标。
type MetricIngestRequest struct {
	Metrics []MetricIngestPoint `json:"metrics"`
}

// MetricIngestPoint 表示单个指标数据点。
type MetricIngestPoint struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// MetricQueryRequest 对应 OpenAPI MetricQueryRequest 结构，支持多种聚合和分组方式。
type MetricQueryRequest struct {
	MetricName  string            `json:"metric_name"`
	TimeRange   TimeRange         `json:"time_range"`
	Aggregation string            `json:"aggregation"` // avg/sum/min/max/count/p50/p90/p95/p99
	Interval    string            `json:"interval"`    // 1m/5m/1h/1d
	Filters     map[string]string `json:"filters,omitempty"`
	GroupBy     []string          `json:"group_by,omitempty"`
}

// TimeRange 表示查询的时间范围。
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// MetricQueryResponse 对应 OpenAPI MetricQueryResponse 结构。
type MetricQueryResponse struct {
	Series []MetricSeries `json:"series"`
}

// MetricSeries 表示指标查询响应中的一条时间序列。
type MetricSeries struct {
	Labels     map[string]string `json:"labels"`
	DataPoints []DataPoint       `json:"data_points"`
}

// DataPoint 是时间序列中的单个数据点（时间戳+值）。
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// QueryRequest 对应 OpenAPI 即席分析查询请求。
type QueryRequest struct {
	Query     string     `json:"query"`
	TimeRange *TimeRange `json:"time_range,omitempty"`
	Limit     int        `json:"limit"`
}

// QueryResponse 对应 OpenAPI 即席查询响应。
type QueryResponse struct {
	Columns         []QueryColumn `json:"columns"`
	Rows            [][]any       `json:"rows"`
	TotalRows       int           `json:"total_rows"`
	ExecutionTimeMs int64         `json:"execution_time_ms"`
}

// QueryColumn 描述查询结果中的一列。
type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ResourceCorrelation 保存两个指标之间的 Pearson 相关系数计算结果。
type ResourceCorrelation struct {
	MetricA         string  `json:"metric_a"`
	MetricB         string  `json:"metric_b"`
	AssetID         string  `json:"asset_id,omitempty"`
	CorrelationCoef float64 `json:"correlation_coefficient"`
	SampleCount     int     `json:"sample_count"`
}

// Dashboard 对应 OpenAPI Dashboard 结构，支持多面板布局。
type Dashboard struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	Panels      []Panel   `json:"panels" db:"panels"`
	OwnerID     string    `json:"owner_id" db:"owner_id"`
	IsPublic    bool      `json:"is_public" db:"is_public"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Panel 对应 OpenAPI Panel 结构，表示仪表盘中的一个可视化面板。
type Panel struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Type     string        `json:"type"` // line_chart/bar_chart/pie_chart/table/stat/heatmap
	Query    string        `json:"query"`
	Position PanelPosition `json:"position"`
}

// PanelPosition 定义面板在仪表盘网格中的位置和大小。
type PanelPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Report 对应 OpenAPI Report 结构，支持定时生成和多格式导出。
type Report struct {
	ID          string     `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description,omitempty" db:"description"`
	Schedule    string     `json:"schedule" db:"schedule"`     // cron expression
	Query       string     `json:"query" db:"query"`           // SQL-like analytics query
	Format      string     `json:"format" db:"format"`         // pdf/csv/json
	Recipients  []string   `json:"recipients" db:"recipients"` // notification targets
	Status      string     `json:"status" db:"status"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty" db:"last_run_at"`
	CreatedBy   string     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// KnowledgeArticle 对应 ON-002 知识库文章实体（FR-17-001 ~ FR-17-008）。
// 支持多类型（案例/Runbook/FAQ/架构/厂商文档）、质量评分和语义搜索。
type KnowledgeArticle struct {
	ArticleID       string    `json:"article_id" db:"article_id"`
	Type            string    `json:"type" db:"type"` // case_study/runbook/faq/architecture/vendor_doc
	Title           string    `json:"title" db:"title"`
	Content         string    `json:"content" db:"content"`
	Tags            []string  `json:"tags" db:"tags"`
	QualityScore    float64   `json:"quality_score" db:"quality_score"`                  // FR-17-006
	RelatedIncident *string   `json:"related_incident,omitempty" db:"related_incident"`  // FR-17-001
	Status          string    `json:"status" db:"status"`
	Embedding       []float32 `json:"-" db:"embedding"` // pgvector for FR-17-003
	CreatedBy       string    `json:"created_by" db:"created_by"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// KnowledgeSearchResult 包装知识库文章及其语义相似度评分。
type KnowledgeSearchResult struct {
	Article    *KnowledgeArticle `json:"article"`
	Similarity float64           `json:"similarity"`
}

// SLAListFilter 定义 SLA 配置列表查询的过滤条件。
type SLAListFilter struct {
	Dimension      *string
	DimensionValue *string
	Page           int
	PageSize       int
}

// ReportListFilter 定义报表列表查询的过滤条件。
type ReportListFilter struct {
	Status   *string
	Page     int
	PageSize int
}

// DashboardListFilter 定义仪表盘列表查询的过滤条件。
type DashboardListFilter struct {
	PageToken string
	PageSize  int
}
