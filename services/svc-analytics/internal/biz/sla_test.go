package biz

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

// --- Mock SLA 仓储 ---

// mockSLARepo 是 SLARepo 的内存模拟实现，用于单元测试。
type mockSLARepo struct {
	configs map[string]*SLAConfig // 内存中的 SLA 配置存储
}

// newMockSLARepo 创建空的模拟 SLA 仓储实例。
func newMockSLARepo() *mockSLARepo {
	return &mockSLARepo{configs: make(map[string]*SLAConfig)}
}

func (m *mockSLARepo) CreateConfig(_ context.Context, cfg *SLAConfig) error {
	cfg.ConfigID = "cfg-" + cfg.Name
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	m.configs[cfg.ConfigID] = cfg
	return nil
}

func (m *mockSLARepo) GetConfig(_ context.Context, id string) (*SLAConfig, error) {
	cfg, ok := m.configs[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return cfg, nil
}

func (m *mockSLARepo) ListConfigs(_ context.Context, filter SLAListFilter) ([]*SLAConfig, int, error) {
	var result []*SLAConfig
	for _, cfg := range m.configs {
		if filter.Dimension != nil && cfg.Dimension != *filter.Dimension {
			continue
		}
		result = append(result, cfg)
	}
	return result, len(result), nil
}

func (m *mockSLARepo) UpdateConfig(_ context.Context, cfg *SLAConfig) error {
	if _, ok := m.configs[cfg.ConfigID]; !ok {
		return fmt.Errorf("not found: %s", cfg.ConfigID)
	}
	m.configs[cfg.ConfigID] = cfg
	return nil
}

func (m *mockSLARepo) DeleteConfig(_ context.Context, id string) error {
	delete(m.configs, id)
	return nil
}

func (m *mockSLARepo) ListConfigsByDimension(_ context.Context, dimension string) ([]*SLAConfig, error) {
	var result []*SLAConfig
	for _, cfg := range m.configs {
		if cfg.Dimension == dimension {
			result = append(result, cfg)
		}
	}
	return result, nil
}

// --- Mock SLA 故障数据仓储 ---

// mockSLAIncidentRepo 是 SLAIncidentRepo 的模拟实现，返回预设的停机时间和事件数量。
type mockSLAIncidentRepo struct {
	downtime      float64 // 预设停机秒数
	incidentCount int     // 预设事件数量
}

func (m *mockSLAIncidentRepo) GetDowntime(_ context.Context, _, _ string, _, _ time.Time, _ bool) (float64, int, error) {
	return m.downtime, m.incidentCount, nil
}

// --- 测试辅助函数 ---

// newTestSLAUsecase 创建测试用 SLA 用例，注入指定的停机时间和事件数量。
func newTestSLAUsecase(downtime float64, incidents int) (*SLAUsecase, *mockSLARepo) {
	repo := newMockSLARepo()
	incidentRepo := &mockSLAIncidentRepo{downtime: downtime, incidentCount: incidents}
	uc := NewSLAUsecase(repo, incidentRepo, nil)
	return uc, repo
}

// seedConfig 创建并持久化测试用 SLA 配置，返回生成的配置 ID。
func seedConfig(t *testing.T, uc *SLAUsecase, name, dimension, dimValue string, target float64) string {
	t.Helper()
	cfg := &SLAConfig{
		Name:             name,
		Dimension:        dimension,
		DimensionValue:   dimValue,
		TargetPercentage: target,
		Window:           "monthly",
		ExcludePlanned:   true,
	}
	if err := uc.CreateConfig(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	return cfg.ConfigID
}

// --- 测试用例 ---

// TestCalculateSLA 验证核心 SLA 计算公式：SLA% = (总时间 - 停机时间) / 总时间 * 100%
// 覆盖场景：零停机、目标边界值、低于目标、大规模停机、精确小量停机、无效时间范围。
func TestCalculateSLA(t *testing.T) {
	// 30 days = 2,592,000 seconds
	totalSeconds := 30.0 * 24 * 3600 // 2592000

	tests := []struct {
		name             string
		downtime         float64
		incidents        int
		target           float64
		wantActual       float64
		wantCompliance   bool
		wantIncidents    int
		actualTolerance  float64
	}{
		{
			name:            "zero downtime = 100% availability",
			downtime:        0,
			incidents:       0,
			target:          99.95,
			wantActual:      100.0,
			wantCompliance:  true,
			wantIncidents:   0,
			actualTolerance: 0.0001,
		},
		{
			name:            "exact target boundary (99.95%)",
			downtime:        totalSeconds * (1.0 - 99.95/100.0), // 1296s
			incidents:       2,
			target:          99.95,
			wantActual:      99.95,
			wantCompliance:  true,
			wantIncidents:   2,
			actualTolerance: 0.001,
		},
		{
			name:            "below target = non-compliant",
			downtime:        totalSeconds * (1.0 - 99.95/100.0), // ~1296s gives 99.95%
			incidents:       3,
			target:          99.99,                               // target higher than actual
			wantActual:      99.95,
			wantCompliance:  false,
			wantIncidents:   3,
			actualTolerance: 0.001,
		},
		{
			name:            "large downtime (1 day out of 30)",
			downtime:        86400,                    // 1 day
			incidents:       5,
			target:          99.9,
			wantActual:      (totalSeconds - 86400) / totalSeconds * 100.0,
			wantCompliance:  false,
			wantIncidents:   5,
			actualTolerance: 0.01,
		},
		{
			name:            "precise calculation with small downtime",
			downtime:        259.2,                                       // 0.01% of 30 days
			incidents:       1,
			target:          99.9,
			wantActual:      (totalSeconds - 259.2) / totalSeconds * 100, // 99.99%
			wantCompliance:  true,
			wantIncidents:   1,
			actualTolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _ := newTestSLAUsecase(tt.downtime, tt.incidents)
			id := seedConfig(t, uc, "sla-"+tt.name, SLALevelBusinessUnit, "payment", tt.target)

			start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

			result, err := uc.Calculate(context.Background(), id, start, end)
			if err != nil {
				t.Fatal(err)
			}

			if math.Abs(result.ActualPercentage-tt.wantActual) > tt.actualTolerance {
				t.Errorf("actual_percentage: want %f (+-%.4f), got %f",
					tt.wantActual, tt.actualTolerance, result.ActualPercentage)
			}
			if result.Compliance != tt.wantCompliance {
				t.Errorf("compliance: want %v, got %v (actual=%f target=%f)",
					tt.wantCompliance, result.Compliance, result.ActualPercentage, result.TargetPercentage)
			}
			if result.IncidentCount != tt.wantIncidents {
				t.Errorf("incident_count: want %d, got %d", tt.wantIncidents, result.IncidentCount)
			}
			if result.DowntimeSeconds != tt.downtime {
				t.Errorf("downtime_seconds: want %f, got %f", tt.downtime, result.DowntimeSeconds)
			}
			if result.TotalSeconds != totalSeconds {
				t.Errorf("total_seconds: want %f, got %f", totalSeconds, result.TotalSeconds)
			}
		})
	}

	// Invalid time range
	t.Run("invalid time range (start > end)", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(0, 0)
		id := seedConfig(t, uc, "inv-sla", SLALevelGlobal, "all", 99.9)

		end := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

		_, err := uc.Calculate(context.Background(), id, start, end)
		if err == nil {
			t.Error("expected error for invalid time range (start > end)")
		}
	})
}

// TestErrorBudget 验证误差预算（Error Budget）剩余百分比的计算。
// 预算公式：budget = total_time * (1 - target/100)
// 剩余比例：max(0, (budget - used) / budget * 100%)
// 覆盖场景：全量剩余、50% 消耗、预算耗尽、微量消耗、高精度目标。
func TestErrorBudget(t *testing.T) {
	// 30 days = 2,592,000 seconds
	totalSeconds := 30.0 * 24 * 3600

	tests := []struct {
		name               string
		downtime           float64
		target             float64
		wantBudgetTotal    float64
		wantBudgetUsed     float64
		wantRemainPctLow   float64 // lower bound (inclusive)
		wantRemainPctHigh  float64 // upper bound (inclusive)
	}{
		{
			name:              "100% budget remaining (zero downtime)",
			downtime:          0,
			target:            99.95,
			wantBudgetTotal:   totalSeconds * 0.0005, // 1296s
			wantBudgetUsed:    0,
			wantRemainPctLow:  100.0,
			wantRemainPctHigh: 100.0,
		},
		{
			name:              "50% budget remaining",
			downtime:          totalSeconds * 0.001 / 2, // half of 99.9% budget = 1296s
			target:            99.9,
			wantBudgetTotal:   totalSeconds * 0.001, // 2592s
			wantBudgetUsed:    totalSeconds * 0.001 / 2,
			wantRemainPctLow:  49.0,
			wantRemainPctHigh: 51.0,
		},
		{
			name:              "budget fully exhausted",
			downtime:          5000,                  // exceeds 2592s budget
			target:            99.9,
			wantBudgetTotal:   totalSeconds * 0.001,  // 2592s
			wantBudgetUsed:    5000,
			wantRemainPctLow:  0,
			wantRemainPctHigh: 0,
		},
		{
			name:              "budget barely consumed (1 second)",
			downtime:          1,
			target:            99.9,
			wantBudgetTotal:   totalSeconds * 0.001,
			wantBudgetUsed:    1,
			wantRemainPctLow:  99.0,
			wantRemainPctHigh: 100.0,
		},
		{
			name:              "99.99% target has very tight budget",
			downtime:          100,                     // 100s of a 259.2s budget
			target:            99.99,
			wantBudgetTotal:   totalSeconds * 0.0001,   // 259.2s
			wantBudgetUsed:    100,
			wantRemainPctLow:  60.0,
			wantRemainPctHigh: 62.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _ := newTestSLAUsecase(tt.downtime, 1)
			id := seedConfig(t, uc, "eb-"+tt.name, SLALevelBusinessUnit, "checkout", tt.target)

			start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

			result, err := uc.Calculate(context.Background(), id, start, end)
			if err != nil {
				t.Fatal(err)
			}

			if math.Abs(result.ErrorBudgetTotal-tt.wantBudgetTotal) > 0.1 {
				t.Errorf("error_budget_total: want %f, got %f", tt.wantBudgetTotal, result.ErrorBudgetTotal)
			}
			if result.ErrorBudgetUsed != tt.wantBudgetUsed {
				t.Errorf("error_budget_used: want %f, got %f", tt.wantBudgetUsed, result.ErrorBudgetUsed)
			}
			if result.ErrorBudgetRemain < tt.wantRemainPctLow || result.ErrorBudgetRemain > tt.wantRemainPctHigh {
				t.Errorf("error_budget_remaining_pct: want [%f, %f], got %f",
					tt.wantRemainPctLow, tt.wantRemainPctHigh, result.ErrorBudgetRemain)
			}
		})
	}
}

// TestSLAByDimension 验证按维度独立计算 SLA 的功能（FR-10-003）。
// 确保各维度（infra_layer、business_unit、asset_grade 等）只返回匹配的配置结果。
func TestSLAByDimension(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	t.Run("business_unit dimension returns only matching configs", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(600, 1)

		seedConfig(t, uc, "payment-sla", SLALevelBusinessUnit, "payment", 99.95)
		seedConfig(t, uc, "order-sla", SLALevelBusinessUnit, "orders", 99.9)
		seedConfig(t, uc, "infra-sla", SLALevelInfraLayer, "host", 99.5) // different dimension

		results, err := uc.CalculateByDimension(context.Background(), SLALevelBusinessUnit, start, end)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 business_unit results, got %d", len(results))
		}
		for _, r := range results {
			if r.Dimension != SLALevelBusinessUnit {
				t.Errorf("expected dimension=%s, got %s", SLALevelBusinessUnit, r.Dimension)
			}
		}
	})

	t.Run("infra_layer dimension returns only matching configs", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(300, 1)

		seedConfig(t, uc, "net-sla", SLALevelInfraLayer, "network", 99.9)
		seedConfig(t, uc, "host-sla", SLALevelInfraLayer, "host", 99.5)
		seedConfig(t, uc, "app-sla", SLALevelInfraLayer, "application", 99.95)
		seedConfig(t, uc, "bu-sla", SLALevelBusinessUnit, "payment", 99.95) // different dimension

		results, err := uc.CalculateByDimension(context.Background(), SLALevelInfraLayer, start, end)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 3 {
			t.Fatalf("expected 3 infra_layer results, got %d", len(results))
		}
		for _, r := range results {
			if r.Dimension != SLALevelInfraLayer {
				t.Errorf("expected dimension=%s, got %s", SLALevelInfraLayer, r.Dimension)
			}
		}
	})

	t.Run("dimension with no configs returns empty slice", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(0, 0)

		seedConfig(t, uc, "some-sla", SLALevelBusinessUnit, "payment", 99.95)

		results, err := uc.CalculateByDimension(context.Background(), SLALevelRegion, start, end)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results for empty dimension, got %d", len(results))
		}
	})

	t.Run("each config gets its own SLA result with correct values", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(600, 2) // 10 min downtime

		seedConfig(t, uc, "s-grade", SLALevelAssetGrade, "S", 99.99)
		seedConfig(t, uc, "a-grade", SLALevelAssetGrade, "A", 99.95)

		results, err := uc.CalculateByDimension(context.Background(), SLALevelAssetGrade, start, end)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 asset_grade results, got %d", len(results))
		}
		for _, r := range results {
			if r.IncidentCount != 2 {
				t.Errorf("expected 2 incidents per result, got %d", r.IncidentCount)
			}
			if r.DowntimeSeconds != 600 {
				t.Errorf("expected 600s downtime per result, got %f", r.DowntimeSeconds)
			}
		}
	})
}

// TestDefaultSLATargets 验证资产等级的默认 SLA 目标映射（FR-10-006）。
// 预期：S→99.99%，A→99.95%，B→99.9%，C→99.5%，D→99.0%
func TestDefaultSLATargets(t *testing.T) {
	expected := map[string]float64{
		"S": 99.99,
		"A": 99.95,
		"B": 99.90,
		"C": 99.50,
		"D": 99.00,
	}

	for grade, target := range expected {
		t.Run("grade_"+grade, func(t *testing.T) {
			got, ok := DefaultSLATargets[grade]
			if !ok {
				t.Fatalf("DefaultSLATargets missing grade %s", grade)
			}
			if got != target {
				t.Errorf("DefaultSLATargets[%s] = %f, want %f", grade, got, target)
			}
		})
	}

	// Ensure no extra grades exist
	if len(DefaultSLATargets) != len(expected) {
		t.Errorf("DefaultSLATargets has %d entries, expected %d", len(DefaultSLATargets), len(expected))
	}
}

// TestSLAValidation 验证 SLA 配置参数校验规则（空名称、无效维度、目标值越界等）。
func TestSLAValidation(t *testing.T) {
	uc, _ := newTestSLAUsecase(0, 0)
	ctx := context.Background()

	tests := []struct {
		name   string
		cfg    SLAConfig
		errMsg string
	}{
		{
			name:   "empty name",
			cfg:    SLAConfig{Dimension: SLALevelGlobal, TargetPercentage: 99.9},
			errMsg: "name is required",
		},
		{
			name:   "invalid dimension",
			cfg:    SLAConfig{Name: "test", Dimension: "invalid", TargetPercentage: 99.9},
			errMsg: "invalid dimension",
		},
		{
			name:   "zero target",
			cfg:    SLAConfig{Name: "test", Dimension: SLALevelGlobal, TargetPercentage: 0},
			errMsg: "target_percentage must be between",
		},
		{
			name:   "target over 100",
			cfg:    SLAConfig{Name: "test", Dimension: SLALevelGlobal, TargetPercentage: 101},
			errMsg: "target_percentage must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uc.CreateConfig(ctx, &tt.cfg)
			if err == nil {
				t.Errorf("expected error containing %q", tt.errMsg)
				return
			}
			if !containsStr(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

// TestCalculateReport 验证按时间段聚合的 SLA 报告生成功能。
// 覆盖场景：每日粒度（30 个周期）、每周粒度（4-5 个周期）、每月粒度（1 个周期）、报告元数据。
func TestCalculateReport(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	t.Run("daily granularity produces 30 periods", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(600, 2)
		id := seedConfig(t, uc, "rpt-daily", SLALevelBusinessUnit, "payment", 99.95)

		report, err := uc.CalculateReport(context.Background(), id, start, end, "daily")
		if err != nil {
			t.Fatal(err)
		}
		if report.Granularity != "daily" {
			t.Errorf("granularity: want daily, got %s", report.Granularity)
		}
		if len(report.Periods) != 30 {
			t.Errorf("expected 30 daily periods, got %d", len(report.Periods))
		}
		if report.OverallActual <= 0 {
			t.Error("overall actual percentage should be positive")
		}
	})

	t.Run("weekly granularity", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(0, 0)
		id := seedConfig(t, uc, "rpt-weekly", SLALevelBusinessUnit, "payment", 99.9)

		report, err := uc.CalculateReport(context.Background(), id, start, end, "weekly")
		if err != nil {
			t.Fatal(err)
		}
		if report.Granularity != "weekly" {
			t.Errorf("granularity: want weekly, got %s", report.Granularity)
		}
		// 30 days / 7 days = ~4-5 periods
		if len(report.Periods) < 4 || len(report.Periods) > 5 {
			t.Errorf("expected 4-5 weekly periods, got %d", len(report.Periods))
		}
		if report.OverallActual != 100.0 {
			t.Errorf("expected 100%% with zero downtime, got %f", report.OverallActual)
		}
		if !report.OverallCompliance {
			t.Error("expected compliant with zero downtime")
		}
	})

	t.Run("monthly granularity produces 1 period", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(100, 1)
		id := seedConfig(t, uc, "rpt-monthly", SLALevelBusinessUnit, "payment", 99.9)

		report, err := uc.CalculateReport(context.Background(), id, start, end, "monthly")
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Periods) != 1 {
			t.Errorf("expected 1 monthly period, got %d", len(report.Periods))
		}
	})

	t.Run("report metadata matches config", func(t *testing.T) {
		uc, _ := newTestSLAUsecase(0, 0)
		id := seedConfig(t, uc, "rpt-meta", SLALevelInfraLayer, "network", 99.99)

		report, err := uc.CalculateReport(context.Background(), id, start, end, "daily")
		if err != nil {
			t.Fatal(err)
		}
		if report.ConfigID != id {
			t.Errorf("config_id: want %s, got %s", id, report.ConfigID)
		}
		if report.Dimension != SLALevelInfraLayer {
			t.Errorf("dimension: want %s, got %s", SLALevelInfraLayer, report.Dimension)
		}
		if report.DimensionValue != "network" {
			t.Errorf("dimension_value: want network, got %s", report.DimensionValue)
		}
		if report.TargetPercentage != 99.99 {
			t.Errorf("target: want 99.99, got %f", report.TargetPercentage)
		}
	})
}

// TestCalculateReport_DailyEdgeCase 验证 CalculateReport 按日粒度的边界场景：
// 跨月时间范围（2 月 28 日 → 3 月 3 日），确保拆分正确且各子周期 SLA 独立计算。
func TestCalculateReport_DailyEdgeCase(t *testing.T) {
	// 非闰年 2 月 28 日 → 3 月 3 日，共 3 天
	start := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)

	// 模拟每个子周期有 60 秒停机
	uc, _ := newTestSLAUsecase(60, 1)
	id := seedConfig(t, uc, "daily-edge", SLALevelBusinessUnit, "payment", 99.9)

	report, err := uc.CalculateReport(context.Background(), id, start, end, "daily")
	if err != nil {
		t.Fatal(err)
	}

	// 2 月 28 → 3 月 1 → 3 月 2 → 3 月 3 = 3 个日周期
	if len(report.Periods) != 3 {
		t.Errorf("跨月按日拆分：期望 3 个周期，实际 %d 个", len(report.Periods))
	}

	// 验证首尾周期的时间边界
	if !report.Periods[0].PeriodStart.Equal(start) {
		t.Errorf("首个周期起始时间应为 %v，实际 %v", start, report.Periods[0].PeriodStart)
	}
	lastPeriod := report.Periods[len(report.Periods)-1]
	if !lastPeriod.PeriodEnd.Equal(end) {
		t.Errorf("末尾周期结束时间应为 %v，实际 %v", end, lastPeriod.PeriodEnd)
	}

	// 验证每个子周期都独立计算了 SLA
	for i, p := range report.Periods {
		if p.TotalSeconds <= 0 {
			t.Errorf("周期 %d 的 TotalSeconds 应大于 0，实际 %f", i, p.TotalSeconds)
		}
		if p.DowntimeSeconds != 60 {
			t.Errorf("周期 %d 的 DowntimeSeconds 期望 60，实际 %f", i, p.DowntimeSeconds)
		}
	}

	// 验证整体达标判定：每日 86400 秒中停机 60 秒 → SLA ≈ 99.93%，对于 99.9% 目标应达标
	if !report.OverallCompliance {
		t.Errorf("整体应达标（actual=%f >= target=%f），但判定为不达标",
			report.OverallActual, report.TargetPercentage)
	}
}

// TestCalculateReport_WeeklyEdgeCase 验证 CalculateReport 按周粒度的边界场景：
// 14 天（恰好 2 个完整周期），验证零停机时 OverallActual 为 100% 且 OverallCompliance 为 true。
func TestCalculateReport_WeeklyEdgeCase(t *testing.T) {
	// 14 天恰好拆分为 2 个完整周期
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	// 零停机场景
	uc, _ := newTestSLAUsecase(0, 0)
	id := seedConfig(t, uc, "weekly-edge", SLALevelInfraLayer, "network", 99.99)

	report, err := uc.CalculateReport(context.Background(), id, start, end, "weekly")
	if err != nil {
		t.Fatal(err)
	}

	// 14 天 / 7 天 = 2 个完整周期
	if len(report.Periods) != 2 {
		t.Errorf("14 天按周拆分：期望 2 个周期，实际 %d 个", len(report.Periods))
	}

	// 零停机时整体 SLA 应为 100%
	if report.OverallActual != 100.0 {
		t.Errorf("零停机时 OverallActual 期望 100.0，实际 %f", report.OverallActual)
	}
	if !report.OverallCompliance {
		t.Error("零停机时 OverallCompliance 应为 true")
	}

	// 验证每个子周期的 DowntimeSeconds 均为 0
	for i, p := range report.Periods {
		if p.DowntimeSeconds != 0 {
			t.Errorf("周期 %d 的 DowntimeSeconds 期望 0，实际 %f", i, p.DowntimeSeconds)
		}
		if p.ActualPercentage != 100.0 {
			t.Errorf("周期 %d 的 ActualPercentage 期望 100.0，实际 %f", i, p.ActualPercentage)
		}
	}
}

// TestCalculateReport_MonthlyEdgeCase 验证 CalculateReport 按月粒度的边界场景：
// 大量停机导致 OverallCompliance 为 false 的不达标场景。
func TestCalculateReport_MonthlyEdgeCase(t *testing.T) {
	// 跨 2 个月的时间范围：1 月 1 日 → 3 月 1 日
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// 每个月子周期停机 86400 秒（1 天），对于 99.99% 目标必然不达标
	uc, _ := newTestSLAUsecase(86400, 5)
	id := seedConfig(t, uc, "monthly-edge", SLALevelAssetGrade, "S", 99.99)

	report, err := uc.CalculateReport(context.Background(), id, start, end, "monthly")
	if err != nil {
		t.Fatal(err)
	}

	// 1 月 → 2 月 → 3 月 1 日 = 2 个月周期
	if len(report.Periods) != 2 {
		t.Errorf("跨 2 个月按月拆分：期望 2 个周期，实际 %d 个", len(report.Periods))
	}

	// 每月停机 1 天 → SLA ≈ 96.7%，远低于 99.99% 目标 → 不达标
	if report.OverallCompliance {
		t.Errorf("大量停机时 OverallCompliance 应为 false（actual=%f < target=%f）",
			report.OverallActual, report.TargetPercentage)
	}

	// 验证整体 SLA 百分比合理性：应远低于 99.99%
	if report.OverallActual >= 99.99 {
		t.Errorf("每月停机 1 天时 OverallActual 应远低于 99.99%%，实际 %f", report.OverallActual)
	}

	// 验证每个子周期都有事件记录
	for i, p := range report.Periods {
		if p.IncidentCount != 5 {
			t.Errorf("周期 %d 的 IncidentCount 期望 5，实际 %d", i, p.IncidentCount)
		}
	}
}

// TestSplitTimePeriods 验证时间范围拆分逻辑。
// 覆盖场景：7 天按日拆分（7 个周期）、7 天按周拆分（1 个周期）、空范围（0 个周期）。
func TestSplitTimePeriods(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)

	t.Run("daily split 7 days", func(t *testing.T) {
		periods := splitTimePeriods(start, end, "daily")
		if len(periods) != 7 {
			t.Errorf("expected 7 daily periods for 7 days, got %d", len(periods))
		}
		// First period should start on March 1
		if !periods[0][0].Equal(start) {
			t.Errorf("first period start: want %v, got %v", start, periods[0][0])
		}
		// Last period should end on March 8
		if !periods[len(periods)-1][1].Equal(end) {
			t.Errorf("last period end: want %v, got %v", end, periods[len(periods)-1][1])
		}
	})

	t.Run("weekly split 7 days", func(t *testing.T) {
		periods := splitTimePeriods(start, end, "weekly")
		if len(periods) != 1 {
			t.Errorf("expected 1 weekly period for 7 days, got %d", len(periods))
		}
	})

	t.Run("empty range", func(t *testing.T) {
		periods := splitTimePeriods(start, start, "daily")
		if len(periods) != 0 {
			t.Errorf("expected 0 periods for zero-length range, got %d", len(periods))
		}
	})
}

// containsStr 检查字符串 s 是否包含子串 substr，用于测试断言。
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
