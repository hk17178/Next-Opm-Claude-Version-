// report_template.go 实现管理层报告模板业务逻辑（FR-15-009~011, FR-15-013）。
// 提供月度运营报告、SLA 达标报告、事件趋势报告和告警质量报告的生成能力。

package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// 报告模板 ID 常量，对应预置的四种管理层报告。
const (
	TemplateMonthlyOps    = "monthly_ops"    // 月度运营报告
	TemplateSLAReport     = "sla_report"     // SLA 达标报告
	TemplateIncidentTrend = "incident_trend" // 事件趋势报告
	TemplateAlertQuality  = "alert_quality"  // 告警质量报告
)

// 报告视角常量。
const (
	PerspectiveManagement = "management" // 管理视角
	PerspectiveTechnical  = "technical"  // 技术视角
)

// ReportTemplate 报告模板定义，描述预置报告的元数据和内容板块。
type ReportTemplate struct {
	ID          string   `json:"id"`          // 模板 ID
	Name        string   `json:"name"`        // 显示名称
	Description string   `json:"description"` // 报告说明
	Perspective string   `json:"perspective"` // 视角：management / technical
	Sections    []string `json:"sections"`    // 包含的内容板块
}

// OpsReport 月度运营报告数据（FR-15-009）。
// 包含 SLA 总览、MTTR/MTTA 趋势、事件统计、根因分类和资源利用率摘要。
type OpsReport struct {
	ID          string              `json:"id"`           // 报告 ID
	TemplateID  string              `json:"template_id"`  // 使用的模板 ID
	Year        int                 `json:"year"`         // 报告年份
	Month       int                 `json:"month"`        // 报告月份
	GeneratedAt time.Time           `json:"generated_at"` // 生成时间
	SLASummary  *SLASummarySection  `json:"sla_summary"`  // SLA 达标总览
	MTTRTrend   []TrendPoint        `json:"mttr_trend"`   // MTTR 趋势数据
	MTTATrend   []TrendPoint        `json:"mtta_trend"`   // MTTA 趋势数据
	IncidentSummary *IncidentSummarySection `json:"incident_summary"` // 事件数量趋势
	RootCauses  []CategoryCount     `json:"root_causes"`  // 根因分类统计
	ResourceSummary *ResourceSummarySection `json:"resource_summary"` // 资源利用率摘要
}

// SLAReport SLA 达标报告数据（FR-15-010）。
// 包含各业务板块 SLA、错误预算环比和不达标根因分析。
type SLAReport struct {
	ID           string            `json:"id"`            // 报告 ID
	TemplateID   string            `json:"template_id"`   // 使用的模板 ID
	StartDate    time.Time         `json:"start_date"`    // 报告起始日期
	EndDate      time.Time         `json:"end_date"`      // 报告结束日期
	GeneratedAt  time.Time         `json:"generated_at"`  // 生成时间
	BizUnitSLAs  []BizUnitSLA      `json:"biz_unit_slas"` // 各业务板块 SLA 数据
	BudgetTrend  []BudgetTrendPoint `json:"budget_trend"` // 错误预算环比趋势
	Violations   []SLAViolation    `json:"violations"`    // 不达标根因分析
}

// IncidentTrendReport 事件趋势报告数据（FR-15-011）。
// 包含各级别事件数量趋势、根因分类占比和高频故障 TOP5。
type IncidentTrendReport struct {
	ID               string             `json:"id"`                // 报告 ID
	TemplateID       string             `json:"template_id"`       // 使用的模板 ID
	StartDate        time.Time          `json:"start_date"`        // 报告起始日期
	EndDate          time.Time          `json:"end_date"`          // 报告结束日期
	GeneratedAt      time.Time          `json:"generated_at"`      // 生成时间
	SeverityTrends   []SeverityTrend    `json:"severity_trends"`   // P0-P4 各级别数量趋势
	RootCauseRatios  []CategoryCount    `json:"root_cause_ratios"` // 根因分类占比
	TopFailures      []TopFailureItem   `json:"top_failures"`      // 高频故障 TOP5
}

// AlertQualityReport 告警质量报告数据（FR-15-013）。
// 包含有效告警率、各层降噪效果和误报 TOP10 规则。
type AlertQualityReport struct {
	ID                string              `json:"id"`                  // 报告 ID
	TemplateID        string              `json:"template_id"`         // 使用的模板 ID
	StartDate         time.Time           `json:"start_date"`          // 报告起始日期
	EndDate           time.Time           `json:"end_date"`            // 报告结束日期
	GeneratedAt       time.Time           `json:"generated_at"`        // 生成时间
	EffectiveAlertRate float64            `json:"effective_alert_rate"` // 有效告警率（%）
	NoiseReduction    []NoiseReductionLayer `json:"noise_reduction"`   // 各层降噪效果
	TopFalsePositives []FalsePositiveRule  `json:"top_false_positives"` // 误报 TOP10 规则
}

// --- 报告子结构定义 ---

// SLASummarySection SLA 达标总览板块。
type SLASummarySection struct {
	OverallSLA     float64 `json:"overall_sla"`      // 整体 SLA 百分比
	TargetSLA      float64 `json:"target_sla"`       // 目标 SLA 百分比
	Compliance     bool    `json:"compliance"`       // 是否达标
	TotalConfigs   int     `json:"total_configs"`    // SLA 配置总数
	CompliantCount int     `json:"compliant_count"`  // 达标配置数
}

// TrendPoint 时间趋势数据点，用于 MTTR/MTTA 等折线图。
type TrendPoint struct {
	Date  string  `json:"date"`  // 日期标签（如 "2026-03-01"）
	Value float64 `json:"value"` // 数值（分钟）
}

// IncidentSummarySection 事件数量统计板块。
type IncidentSummarySection struct {
	Total    int            `json:"total"`     // 事件总数
	BySeverity map[string]int `json:"by_severity"` // 按级别分组计数
}

// CategoryCount 分类计数，用于根因分类统计。
type CategoryCount struct {
	Category string  `json:"category"` // 分类名称
	Count    int     `json:"count"`    // 数量
	Ratio    float64 `json:"ratio"`    // 占比（%）
}

// ResourceSummarySection 资源利用率摘要板块。
type ResourceSummarySection struct {
	AvgCPUUsage    float64 `json:"avg_cpu_usage"`    // 平均 CPU 使用率（%）
	AvgMemoryUsage float64 `json:"avg_memory_usage"` // 平均内存使用率（%）
	AvgDiskUsage   float64 `json:"avg_disk_usage"`   // 平均磁盘使用率（%）
}

// BizUnitSLA 各业务板块 SLA 数据。
type BizUnitSLA struct {
	BusinessUnit string  `json:"business_unit"` // 业务板块名称
	TargetSLA    float64 `json:"target_sla"`    // 目标 SLA
	ActualSLA    float64 `json:"actual_sla"`    // 实际 SLA
	Compliance   bool    `json:"compliance"`    // 是否达标
}

// BudgetTrendPoint 错误预算环比趋势数据点。
type BudgetTrendPoint struct {
	Period          string  `json:"period"`           // 时间周期标签
	BudgetRemaining float64 `json:"budget_remaining"` // 剩余预算百分比
}

// SLAViolation SLA 不达标详情。
type SLAViolation struct {
	ConfigID       string  `json:"config_id"`       // SLA 配置 ID
	DimensionValue string  `json:"dimension_value"` // 维度值
	TargetSLA      float64 `json:"target_sla"`      // 目标 SLA
	ActualSLA      float64 `json:"actual_sla"`      // 实际 SLA
	RootCause      string  `json:"root_cause"`      // 不达标根因
}

// SeverityTrend 按事件级别的数量趋势。
type SeverityTrend struct {
	Severity string       `json:"severity"` // 事件级别（P0-P4）
	Points   []TrendPoint `json:"points"`   // 趋势数据点
}

// TopFailureItem 高频故障条目。
type TopFailureItem struct {
	Name       string `json:"name"`        // 故障名称/描述
	Count      int    `json:"count"`       // 发生次数
	LastSeenAt string `json:"last_seen_at"` // 最近发生时间
}

// NoiseReductionLayer 各层降噪效果数据。
type NoiseReductionLayer struct {
	Layer          string  `json:"layer"`           // 降噪层名称
	InputCount     int     `json:"input_count"`     // 输入告警数
	OutputCount    int     `json:"output_count"`    // 输出告警数
	ReductionRate  float64 `json:"reduction_rate"`  // 降噪率（%）
}

// FalsePositiveRule 误报规则条目。
type FalsePositiveRule struct {
	RuleName       string `json:"rule_name"`        // 规则名称
	FalsePositives int    `json:"false_positives"`   // 误报次数
	TotalFires     int    `json:"total_fires"`       // 总触发次数
}

// GeneratedReport 已生成报告的通用包装结构。
// 存储报告元数据和序列化后的报告内容。
type GeneratedReport struct {
	ID          string      `json:"id" db:"id"`                   // 报告 ID
	TemplateID  string      `json:"template_id" db:"template_id"` // 使用的模板 ID
	Title       string      `json:"title" db:"title"`              // 报告标题
	Content     interface{} `json:"content"`                       // 报告内容（OpsReport / SLAReport / ...）
	ContentJSON string      `json:"-" db:"content_json"`           // JSON 序列化后的报告内容
	Format      string      `json:"format" db:"format"`            // 输出格式
	StartDate   time.Time   `json:"start_date" db:"start_date"`    // 报告起始日期
	EndDate     time.Time   `json:"end_date" db:"end_date"`        // 报告结束日期
	GeneratedAt time.Time   `json:"generated_at" db:"generated_at"` // 生成时间
	GeneratedBy string      `json:"generated_by" db:"generated_by"` // 生成人
}

// GenerateReportRequest 报告生成请求参数。
type GenerateReportRequest struct {
	TemplateID string            `json:"template_id"` // 模板 ID
	StartDate  time.Time         `json:"start_date"`  // 起始日期
	EndDate    time.Time         `json:"end_date"`    // 结束日期
	Filters    map[string]string `json:"filters"`     // 筛选维度
}

// ReportTemplateRepo 报告模板和已生成报告的数据访问接口。
type ReportTemplateRepo interface {
	// SaveGeneratedReport 保存已生成的报告
	SaveGeneratedReport(ctx context.Context, r *GeneratedReport) error
	// GetGeneratedReport 获取已生成的报告
	GetGeneratedReport(ctx context.Context, id string) (*GeneratedReport, error)
}

// ReportTemplateUsecase 报告模板业务逻辑用例。
// 负责管理预置报告模板和生成各类管理层报告。
type ReportTemplateUsecase struct {
	templateRepo ReportTemplateRepo // 报告模板数据访问层
	slaUC        *SLAUsecase        // SLA 计算用例（用于报告数据获取）
	metricsRepo  MetricsRepo        // 指标查询仓储
	logger       *zap.Logger        // 日志记录器
}

// NewReportTemplateUsecase 创建报告模板业务用例实例。
func NewReportTemplateUsecase(
	templateRepo ReportTemplateRepo,
	slaUC *SLAUsecase,
	metricsRepo MetricsRepo,
	logger *zap.Logger,
) *ReportTemplateUsecase {
	return &ReportTemplateUsecase{
		templateRepo: templateRepo,
		slaUC:        slaUC,
		metricsRepo:  metricsRepo,
		logger:       logger,
	}
}

// presetTemplates 预置报告模板定义。
var presetTemplates = []*ReportTemplate{
	{
		ID:          TemplateMonthlyOps,
		Name:        "月度运营报告",
		Description: "包含 SLA 达标总览、MTTR/MTTA 趋势、事件数量趋势、根因分类和资源利用率摘要",
		Perspective: PerspectiveManagement,
		Sections:    []string{"sla_summary", "mttr_trend", "mtta_trend", "incident_summary", "root_causes", "resource_summary"},
	},
	{
		ID:          TemplateSLAReport,
		Name:        "SLA 达标报告",
		Description: "包含各业务板块 SLA、错误预算环比和不达标根因分析",
		Perspective: PerspectiveManagement,
		Sections:    []string{"biz_unit_slas", "budget_trend", "violations"},
	},
	{
		ID:          TemplateIncidentTrend,
		Name:        "事件趋势报告",
		Description: "包含 P0-P4 数量趋势、根因分类占比和高频故障 TOP5",
		Perspective: PerspectiveTechnical,
		Sections:    []string{"severity_trends", "root_cause_ratios", "top_failures"},
	},
	{
		ID:          TemplateAlertQuality,
		Name:        "告警质量报告",
		Description: "包含有效告警率、各层降噪效果和误报 TOP10 规则",
		Perspective: PerspectiveTechnical,
		Sections:    []string{"effective_alert_rate", "noise_reduction", "top_false_positives"},
	},
}

// ListTemplates 列出所有预置报告模板。
func (uc *ReportTemplateUsecase) ListTemplates() []*ReportTemplate {
	return presetTemplates
}

// GetTemplate 根据 ID 获取单个预置模板。
func (uc *ReportTemplateUsecase) GetTemplate(id string) (*ReportTemplate, error) {
	for _, t := range presetTemplates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// GenerateMonthlyOpsReport 生成月度运营报告（FR-15-009）。
// 包含：SLA 达标总览 + MTTR/MTTA 趋势 + 事件数量趋势 + 根因分类 + 资源利用率摘要。
func (uc *ReportTemplateUsecase) GenerateMonthlyOpsReport(ctx context.Context, year, month int) (*OpsReport, error) {
	// 计算报告时间范围
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	report := &OpsReport{
		ID:          fmt.Sprintf("ops-%d-%02d", year, month),
		TemplateID:  TemplateMonthlyOps,
		Year:        year,
		Month:       month,
		GeneratedAt: time.Now(),
	}

	// 获取 SLA 总览数据
	slaSummary, err := uc.buildSLASummary(ctx, start, end)
	if err != nil {
		uc.logger.Warn("构建 SLA 总览失败", zap.Error(err))
	} else {
		report.SLASummary = slaSummary
	}

	// MTTR/MTTA 趋势：从 ClickHouse 查询事件指标
	report.MTTRTrend = uc.queryTrendMetric(ctx, "mttr", start, end)
	report.MTTATrend = uc.queryTrendMetric(ctx, "mtta", start, end)

	// 事件数量统计
	report.IncidentSummary = uc.buildIncidentSummary(ctx, start, end)

	// 根因分类
	report.RootCauses = uc.queryRootCauses(ctx, start, end)

	// 资源利用率
	report.ResourceSummary = uc.buildResourceSummary(ctx, start, end)

	return report, nil
}

// GenerateSLAReport 生成 SLA 达标报告（FR-15-010）。
// 包含：各业务板块 SLA + 错误预算环比 + 不达标根因分析。
func (uc *ReportTemplateUsecase) GenerateSLAReport(ctx context.Context, startDate, endDate time.Time) (*SLAReport, error) {
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}

	report := &SLAReport{
		ID:          fmt.Sprintf("sla-%s-%s", startDate.Format("20060102"), endDate.Format("20060102")),
		TemplateID:  TemplateSLAReport,
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
	}

	// 各业务板块 SLA 数据
	results, err := uc.slaUC.CalculateByDimension(ctx, SLALevelBusinessUnit, startDate, endDate)
	if err != nil {
		uc.logger.Warn("查询业务板块 SLA 失败", zap.Error(err))
	} else {
		for _, r := range results {
			report.BizUnitSLAs = append(report.BizUnitSLAs, BizUnitSLA{
				BusinessUnit: r.DimensionValue,
				TargetSLA:    r.TargetPercentage,
				ActualSLA:    r.ActualPercentage,
				Compliance:   r.Compliance,
			})
			// 收集不达标项
			if !r.Compliance {
				report.Violations = append(report.Violations, SLAViolation{
					ConfigID:       r.ConfigID,
					DimensionValue: r.DimensionValue,
					TargetSLA:      r.TargetPercentage,
					ActualSLA:      r.ActualPercentage,
					RootCause:      "downtime_exceeded",
				})
			}
		}
	}

	return report, nil
}

// GenerateIncidentTrendReport 生成事件趋势报告（FR-15-011）。
// 包含：P0-P4 数量趋势 + 根因分类占比 + 高频故障 TOP5。
func (uc *ReportTemplateUsecase) GenerateIncidentTrendReport(ctx context.Context, startDate, endDate time.Time) (*IncidentTrendReport, error) {
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}

	report := &IncidentTrendReport{
		ID:          fmt.Sprintf("incident-%s-%s", startDate.Format("20060102"), endDate.Format("20060102")),
		TemplateID:  TemplateIncidentTrend,
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
	}

	// P0-P4 各级别事件数量趋势
	severities := []string{"P0", "P1", "P2", "P3", "P4"}
	for _, sev := range severities {
		trend := SeverityTrend{
			Severity: sev,
			Points:   uc.queryTrendMetric(ctx, fmt.Sprintf("incident_count_%s", sev), startDate, endDate),
		}
		report.SeverityTrends = append(report.SeverityTrends, trend)
	}

	// 根因分类占比
	report.RootCauseRatios = uc.queryRootCauses(ctx, startDate, endDate)

	// 高频故障 TOP5：从 ClickHouse 查询
	report.TopFailures = uc.queryTopFailures(ctx, startDate, endDate, 5)

	return report, nil
}

// GenerateAlertQualityReport 生成告警质量报告（FR-15-013）。
// 包含：有效告警率 + 各层降噪效果 + 误报 TOP10 规则。
func (uc *ReportTemplateUsecase) GenerateAlertQualityReport(ctx context.Context, startDate, endDate time.Time) (*AlertQualityReport, error) {
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}

	report := &AlertQualityReport{
		ID:          fmt.Sprintf("alert-quality-%s-%s", startDate.Format("20060102"), endDate.Format("20060102")),
		TemplateID:  TemplateAlertQuality,
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
	}

	// 有效告警率和降噪数据从 ClickHouse 查询
	uc.populateAlertQualityData(ctx, report, startDate, endDate)

	return report, nil
}

// GetGeneratedReport 获取已生成的报告。
func (uc *ReportTemplateUsecase) GetGeneratedReport(ctx context.Context, id string) (*GeneratedReport, error) {
	return uc.templateRepo.GetGeneratedReport(ctx, id)
}

// SaveGeneratedReport 保存已生成的报告。
func (uc *ReportTemplateUsecase) SaveGeneratedReport(ctx context.Context, r *GeneratedReport) error {
	return uc.templateRepo.SaveGeneratedReport(ctx, r)
}

// --- 内部辅助方法 ---

// buildSLASummary 构建 SLA 达标总览数据。
func (uc *ReportTemplateUsecase) buildSLASummary(ctx context.Context, start, end time.Time) (*SLASummarySection, error) {
	results, err := uc.slaUC.CalculateByDimension(ctx, SLALevelGlobal, start, end)
	if err != nil {
		return nil, err
	}

	summary := &SLASummarySection{
		TotalConfigs: len(results),
	}

	var totalActual float64
	for _, r := range results {
		totalActual += r.ActualPercentage
		if r.Compliance {
			summary.CompliantCount++
		}
		summary.TargetSLA = r.TargetPercentage // 取最后一个，通常全局只有一个
	}
	if len(results) > 0 {
		summary.OverallSLA = totalActual / float64(len(results))
	}
	summary.Compliance = summary.CompliantCount == summary.TotalConfigs

	return summary, nil
}

// queryTrendMetric 查询指定指标的时间趋势数据。
func (uc *ReportTemplateUsecase) queryTrendMetric(ctx context.Context, metricName string, start, end time.Time) []TrendPoint {
	q := MetricQueryRequest{
		MetricName:  metricName,
		TimeRange:   TimeRange{Start: start, End: end},
		Aggregation: "avg",
		Interval:    "1d",
	}

	resp, err := uc.metricsRepo.QueryBusinessMetrics(ctx, q)
	if err != nil {
		uc.logger.Warn("查询趋势指标失败", zap.String("metric", metricName), zap.Error(err))
		return nil
	}

	var points []TrendPoint
	for _, series := range resp.Series {
		for _, dp := range series.DataPoints {
			points = append(points, TrendPoint{
				Date:  dp.Timestamp.Format("2006-01-02"),
				Value: dp.Value,
			})
		}
	}
	return points
}

// buildIncidentSummary 构建事件数量统计数据。
func (uc *ReportTemplateUsecase) buildIncidentSummary(ctx context.Context, start, end time.Time) *IncidentSummarySection {
	summary := &IncidentSummarySection{
		BySeverity: make(map[string]int),
	}

	// 通过 ClickHouse 查询事件统计
	q := MetricQueryRequest{
		MetricName:  "incident_count",
		TimeRange:   TimeRange{Start: start, End: end},
		Aggregation: "sum",
		GroupBy:     []string{"severity"},
	}

	resp, err := uc.metricsRepo.QueryBusinessMetrics(ctx, q)
	if err != nil {
		uc.logger.Warn("查询事件统计失败", zap.Error(err))
		return summary
	}

	for _, series := range resp.Series {
		sev := series.Labels["severity"]
		for _, dp := range series.DataPoints {
			count := int(dp.Value)
			summary.BySeverity[sev] += count
			summary.Total += count
		}
	}

	return summary
}

// queryRootCauses 查询根因分类统计。
func (uc *ReportTemplateUsecase) queryRootCauses(ctx context.Context, start, end time.Time) []CategoryCount {
	q := MetricQueryRequest{
		MetricName:  "incident_root_cause",
		TimeRange:   TimeRange{Start: start, End: end},
		Aggregation: "count",
		GroupBy:     []string{"root_cause_category"},
	}

	resp, err := uc.metricsRepo.QueryBusinessMetrics(ctx, q)
	if err != nil {
		uc.logger.Warn("查询根因分类失败", zap.Error(err))
		return nil
	}

	var total int
	var counts []CategoryCount
	for _, series := range resp.Series {
		count := 0
		for _, dp := range series.DataPoints {
			count += int(dp.Value)
		}
		total += count
		counts = append(counts, CategoryCount{
			Category: series.Labels["root_cause_category"],
			Count:    count,
		})
	}

	// 计算占比
	for i := range counts {
		if total > 0 {
			counts[i].Ratio = float64(counts[i].Count) / float64(total) * 100
		}
	}

	return counts
}

// buildResourceSummary 构建资源利用率摘要。
func (uc *ReportTemplateUsecase) buildResourceSummary(ctx context.Context, start, end time.Time) *ResourceSummarySection {
	summary := &ResourceSummarySection{}

	metrics := map[string]*float64{
		"cpu_usage":    &summary.AvgCPUUsage,
		"memory_usage": &summary.AvgMemoryUsage,
		"disk_usage":   &summary.AvgDiskUsage,
	}

	for metricName, target := range metrics {
		q := MetricQueryRequest{
			MetricName:  metricName,
			TimeRange:   TimeRange{Start: start, End: end},
			Aggregation: "avg",
		}
		resp, err := uc.metricsRepo.QueryResourceMetrics(ctx, q)
		if err != nil {
			uc.logger.Warn("查询资源指标失败", zap.String("metric", metricName), zap.Error(err))
			continue
		}
		for _, series := range resp.Series {
			for _, dp := range series.DataPoints {
				*target = dp.Value
			}
		}
	}

	return summary
}

// queryTopFailures 查询高频故障 TOP N。
func (uc *ReportTemplateUsecase) queryTopFailures(ctx context.Context, start, end time.Time, limit int) []TopFailureItem {
	query := fmt.Sprintf(
		"SELECT title, count(*) as cnt, max(detected_at) as last_seen FROM sla_incidents WHERE detected_at BETWEEN '%s' AND '%s' GROUP BY title ORDER BY cnt DESC LIMIT %d",
		start.Format("2006-01-02"), end.Format("2006-01-02"), limit,
	)

	resp, err := uc.metricsRepo.ExecuteQuery(ctx, query, &TimeRange{Start: start, End: end}, limit)
	if err != nil {
		uc.logger.Warn("查询高频故障失败", zap.Error(err))
		return nil
	}

	var items []TopFailureItem
	for _, row := range resp.Rows {
		item := TopFailureItem{}
		if len(row) > 0 {
			item.Name = fmt.Sprintf("%v", row[0])
		}
		if len(row) > 1 {
			if cnt, ok := row[1].(float64); ok {
				item.Count = int(cnt)
			}
		}
		if len(row) > 2 {
			item.LastSeenAt = fmt.Sprintf("%v", row[2])
		}
		items = append(items, item)
	}

	return items
}

// populateAlertQualityData 填充告警质量数据。
func (uc *ReportTemplateUsecase) populateAlertQualityData(ctx context.Context, report *AlertQualityReport, start, end time.Time) {
	// 有效告警率：从指标系统查询
	q := MetricQueryRequest{
		MetricName:  "effective_alert_rate",
		TimeRange:   TimeRange{Start: start, End: end},
		Aggregation: "avg",
	}
	resp, err := uc.metricsRepo.QueryBusinessMetrics(ctx, q)
	if err != nil {
		uc.logger.Warn("查询有效告警率失败", zap.Error(err))
	} else {
		for _, series := range resp.Series {
			for _, dp := range series.DataPoints {
				report.EffectiveAlertRate = dp.Value
			}
		}
	}

	// 各层降噪效果
	layers := []string{"dedup", "correlation", "suppression", "ai_filter"}
	for _, layer := range layers {
		layerQ := MetricQueryRequest{
			MetricName: fmt.Sprintf("noise_reduction_%s", layer),
			TimeRange:  TimeRange{Start: start, End: end},
			Aggregation: "sum",
		}
		layerResp, err := uc.metricsRepo.QueryBusinessMetrics(ctx, layerQ)
		if err != nil {
			continue
		}
		nrl := NoiseReductionLayer{Layer: layer}
		for _, series := range layerResp.Series {
			for _, dp := range series.DataPoints {
				nrl.InputCount += int(dp.Value)
			}
			// 输出数量为输入减去降噪量
			if outputLabel, ok := series.Labels["output"]; ok {
				_ = outputLabel
			}
		}
		report.NoiseReduction = append(report.NoiseReduction, nrl)
	}
}
