package biz

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
)

// SLARepo 定义 SLA 配置的数据访问接口（PostgreSQL 存储）。
type SLARepo interface {
	CreateConfig(ctx context.Context, cfg *SLAConfig) error
	GetConfig(ctx context.Context, id string) (*SLAConfig, error)
	ListConfigs(ctx context.Context, filter SLAListFilter) ([]*SLAConfig, int, error)
	UpdateConfig(ctx context.Context, cfg *SLAConfig) error
	DeleteConfig(ctx context.Context, id string) error
	ListConfigsByDimension(ctx context.Context, dimension string) ([]*SLAConfig, error)
}

// SLAIncidentRepo 定义 SLA 故障数据的数据访问接口（ClickHouse 存储）。
type SLAIncidentRepo interface {
	// GetDowntime 返回指定维度/值在时间范围内的总非计划停机秒数和事件数量。
	// 当 excludePlanned 为 true 时，排除计划性维护（is_planned=1）的停机时间。
	GetDowntime(ctx context.Context, dimension, dimensionValue string, start, end time.Time, excludePlanned bool) (downtimeSeconds float64, incidentCount int, err error)
}

// SLAUsecase 实现多层级 SLA 计算业务逻辑。
//
// 核心公式（FR-10-001）：
//
//	SLA% = (总时间 - 非计划停机时间) / 总时间 * 100%
//
// 支持 FR-10-003 任意维度组合计算（基础设施层、业务单元、资产组、等级、区域等）。
//
// 误差预算管理（FR-10-004）：
//
//	错误预算总量 = 总时间 * (1 - 目标百分比/100)
//	剩余百分比 = max(0, (预算总量 - 已用) / 预算总量 * 100%)
//
// 当剩余预算低于阈值时，通过 EventProducer 发布告警事件到 Kafka。
type SLAUsecase struct {
	slaRepo      SLARepo
	incidentRepo SLAIncidentRepo
	logger       *zap.Logger
	producer     EventProducer
}

// NewSLAUsecase 创建 SLA 用例实例。
// 可选的 EventProducer 参数用于启用误差预算告警推送。
func NewSLAUsecase(slaRepo SLARepo, incidentRepo SLAIncidentRepo, logger *zap.Logger, producers ...EventProducer) *SLAUsecase {
	uc := &SLAUsecase{
		slaRepo:      slaRepo,
		incidentRepo: incidentRepo,
		logger:       logger,
	}
	if len(producers) > 0 && producers[0] != nil {
		uc.producer = producers[0]
	}
	return uc
}

// CreateConfig 创建新的 SLA 配置，写入前进行参数校验。
func (uc *SLAUsecase) CreateConfig(ctx context.Context, cfg *SLAConfig) error {
	if err := validateSLAConfig(cfg); err != nil {
		return err
	}
	return uc.slaRepo.CreateConfig(ctx, cfg)
}

// GetConfig 根据 ID 获取单个 SLA 配置。
func (uc *SLAUsecase) GetConfig(ctx context.Context, id string) (*SLAConfig, error) {
	return uc.slaRepo.GetConfig(ctx, id)
}

// ListConfigs 分页返回 SLA 配置列表。
func (uc *SLAUsecase) ListConfigs(ctx context.Context, filter SLAListFilter) ([]*SLAConfig, int, error) {
	return uc.slaRepo.ListConfigs(ctx, filter)
}

// UpdateConfig 更新已有的 SLA 配置，更新前进行参数校验。
func (uc *SLAUsecase) UpdateConfig(ctx context.Context, cfg *SLAConfig) error {
	if err := validateSLAConfig(cfg); err != nil {
		return err
	}
	return uc.slaRepo.UpdateConfig(ctx, cfg)
}

// DeleteConfig 删除指定的 SLA 配置。
func (uc *SLAUsecase) DeleteConfig(ctx context.Context, id string) error {
	return uc.slaRepo.DeleteConfig(ctx, id)
}

// Calculate 计算指定配置和时间段的 SLA 结果。
// 包含 SLA 百分比计算（FR-10-001）和误差预算计算（FR-10-004），
// 并在预算不足时自动触发告警事件。
func (uc *SLAUsecase) Calculate(ctx context.Context, configID string, start, end time.Time) (*SLAResult, error) {
	cfg, err := uc.slaRepo.GetConfig(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("get sla config: %w", err)
	}

	totalSeconds := end.Sub(start).Seconds()
	if totalSeconds <= 0 {
		return nil, fmt.Errorf("invalid time range: end must be after start")
	}

	downtimeSeconds, incidentCount, err := uc.incidentRepo.GetDowntime(
		ctx, cfg.Dimension, cfg.DimensionValue, start, end, cfg.ExcludePlanned,
	)
	if err != nil {
		return nil, fmt.Errorf("get downtime: %w", err)
	}

	// FR-10-001: SLA = (total - downtime) / total * 100
	actual := (totalSeconds - downtimeSeconds) / totalSeconds * 100.0
	actual = math.Round(actual*10000) / 10000

	// FR-10-004: Error budget
	errorBudgetTotal := totalSeconds * (1.0 - cfg.TargetPercentage/100.0)
	errorBudgetUsed := downtimeSeconds
	errorBudgetRemainPct := 0.0
	if errorBudgetTotal > 0 {
		errorBudgetRemainPct = math.Max(0, (errorBudgetTotal-errorBudgetUsed)/errorBudgetTotal*100.0)
		errorBudgetRemainPct = math.Round(errorBudgetRemainPct*100) / 100
	}

	result := &SLAResult{
		ConfigID:          cfg.ConfigID,
		Dimension:         cfg.Dimension,
		DimensionValue:    cfg.DimensionValue,
		TargetPercentage:  cfg.TargetPercentage,
		ActualPercentage:  actual,
		Compliance:        actual >= cfg.TargetPercentage,
		PeriodStart:       start,
		PeriodEnd:         end,
		TotalSeconds:      totalSeconds,
		DowntimeSeconds:   downtimeSeconds,
		ErrorBudgetTotal:  errorBudgetTotal,
		ErrorBudgetUsed:   errorBudgetUsed,
		ErrorBudgetRemain: errorBudgetRemainPct,
		IncidentCount:     incidentCount,
	}

	if uc.producer != nil {
		if evt := CheckBudgetAlert(result); evt != nil {
			if err := uc.producer.Publish(ctx, "analytics.error_budget_alert", evt); err != nil {
				uc.logger.Warn("failed to publish budget alert", zap.Error(err))
			} else {
				uc.logger.Info("error budget alert fired",
					zap.String("config_id", result.ConfigID),
					zap.Float64("remain_pct", result.ErrorBudgetRemain),
					zap.String("severity", evt.Severity),
				)
			}
		}
	}

	return result, nil
}

// CalculateByDimension 计算指定维度下所有配置的 SLA 结果。
// 单个配置计算失败不影响其他配置，错误会被记录但不中断整体流程。
func (uc *SLAUsecase) CalculateByDimension(ctx context.Context, dimension string, start, end time.Time) ([]*SLAResult, error) {
	configs, err := uc.slaRepo.ListConfigsByDimension(ctx, dimension)
	if err != nil {
		return nil, fmt.Errorf("list configs by dimension: %w", err)
	}

	results := make([]*SLAResult, 0, len(configs))
	for _, cfg := range configs {
		result, err := uc.Calculate(ctx, cfg.ConfigID, start, end)
		if err != nil {
			uc.logger.Warn("sla calculation failed",
				zap.String("config_id", cfg.ConfigID),
				zap.Error(err),
			)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// SLAPeriodReport 按时间段聚合的 SLA 报告，包含各周期的 SLA 计算结果和整体达标情况。
type SLAPeriodReport struct {
	ConfigID          string      `json:"config_id"`                // SLA 配置 ID
	Dimension         string      `json:"dimension"`                // 计算维度（如 business_unit、infra_layer）
	DimensionValue    string      `json:"dimension_value"`          // 维度值（如 payment、network）
	TargetPercentage  float64     `json:"target_percentage"`        // SLA 目标百分比
	Granularity       string      `json:"granularity"`              // 时间粒度：daily / weekly / monthly
	Periods           []SLAResult `json:"periods"`                  // 各子周期的 SLA 计算结果
	OverallActual     float64     `json:"overall_actual_percentage"` // 整体时间段的实际 SLA 百分比
	OverallCompliance bool        `json:"overall_compliance"`       // 整体是否达标（actual >= target）
}

// CalculateReport 生成按时间段（日/周/月）聚合的 SLA 报告。
// granularity 支持 "daily"（按天）、"weekly"（按周）、"monthly"（按月）。
// 流程：先将时间范围按粒度拆分为子周期，逐个计算 SLA，最后汇总整体达标率。
func (uc *SLAUsecase) CalculateReport(ctx context.Context, configID string, start, end time.Time, granularity string) (*SLAPeriodReport, error) {
	// 获取 SLA 配置
	cfg, err := uc.slaRepo.GetConfig(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("get sla config: %w", err)
	}

	// 按粒度拆分时间范围为子周期
	periods := splitTimePeriods(start, end, granularity)
	if len(periods) == 0 {
		return nil, fmt.Errorf("invalid time range or granularity")
	}

	// 构建报告元数据
	report := &SLAPeriodReport{
		ConfigID:         cfg.ConfigID,
		Dimension:        cfg.Dimension,
		DimensionValue:   cfg.DimensionValue,
		TargetPercentage: cfg.TargetPercentage,
		Granularity:      granularity,
	}

	// 逐子周期计算 SLA，累计停机时间和总时间用于整体汇总
	var totalDowntime, totalSeconds float64
	for _, p := range periods {
		result, err := uc.Calculate(ctx, configID, p[0], p[1])
		if err != nil {
			// 单个子周期计算失败不中断整体，记录警告后跳过
			if uc.logger != nil {
				uc.logger.Warn("sla period calculation failed",
					zap.String("config_id", configID),
					zap.Time("start", p[0]),
					zap.Time("end", p[1]),
					zap.Error(err),
				)
			}
			continue
		}
		report.Periods = append(report.Periods, *result)
		totalDowntime += result.DowntimeSeconds
		totalSeconds += result.TotalSeconds
	}

	// 计算整体 SLA 百分比和达标状态
	if totalSeconds > 0 {
		report.OverallActual = math.Round((totalSeconds-totalDowntime)/totalSeconds*100.0*10000) / 10000
		report.OverallCompliance = report.OverallActual >= cfg.TargetPercentage
	}

	return report, nil
}

// splitTimePeriods 将时间范围按指定粒度（daily/weekly/monthly）拆分为子周期数组。
// 最后一个周期的结束时间不会超过 end，确保完整覆盖原始范围。
func splitTimePeriods(start, end time.Time, granularity string) [][2]time.Time {
	var periods [][2]time.Time
	current := start

	for current.Before(end) {
		var next time.Time
		switch granularity {
		case "daily":
			next = current.AddDate(0, 0, 1)
		case "weekly":
			next = current.AddDate(0, 0, 7)
		case "monthly":
			next = current.AddDate(0, 1, 0)
		default:
			next = current.AddDate(0, 1, 0)
		}
		if next.After(end) {
			next = end
		}
		periods = append(periods, [2]time.Time{current, next})
		current = next
	}

	return periods
}

// validateSLAConfig 校验 SLA 配置参数的合法性。
func validateSLAConfig(cfg *SLAConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch cfg.Dimension {
	case SLALevelInfraLayer, SLALevelBusinessUnit, SLALevelAssetGroup,
		SLALevelAssetGrade, SLALevelRegion, SLALevelAsset, SLALevelGlobal:
	default:
		return fmt.Errorf("invalid dimension: %s", cfg.Dimension)
	}
	if cfg.TargetPercentage <= 0 || cfg.TargetPercentage > 100 {
		return fmt.Errorf("target_percentage must be between 0 and 100")
	}
	return nil
}
