package biz

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// --- Mock 报告模板仓储 ---

// mockReportTemplateRepo 是 ReportTemplateRepo 的内存模拟实现。
type mockReportTemplateRepo struct {
	reports map[string]*GeneratedReport
}

func newMockReportTemplateRepo() *mockReportTemplateRepo {
	return &mockReportTemplateRepo{
		reports: make(map[string]*GeneratedReport),
	}
}

func (m *mockReportTemplateRepo) SaveGeneratedReport(_ context.Context, r *GeneratedReport) error {
	m.reports[r.ID] = r
	return nil
}

func (m *mockReportTemplateRepo) GetGeneratedReport(_ context.Context, id string) (*GeneratedReport, error) {
	r, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return r, nil
}

// --- Mock 指标仓储（用于报告模板测试） ---

// mockReportMetricsRepo 是 MetricsRepo 的模拟实现，返回预设数据。
type mockReportMetricsRepo struct{}

func (m *mockReportMetricsRepo) InsertBusinessMetrics(_ context.Context, _ []BusinessMetric) error {
	return nil
}

func (m *mockReportMetricsRepo) InsertResourceMetrics(_ context.Context, _ []ResourceMetric) error {
	return nil
}

func (m *mockReportMetricsRepo) QueryBusinessMetrics(_ context.Context, q MetricQueryRequest) (*MetricQueryResponse, error) {
	return &MetricQueryResponse{
		Series: []MetricSeries{
			{
				Labels: map[string]string{"severity": "P0"},
				DataPoints: []DataPoint{
					{Timestamp: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Value: 5.0},
				},
			},
		},
	}, nil
}

func (m *mockReportMetricsRepo) QueryResourceMetrics(_ context.Context, _ MetricQueryRequest) (*MetricQueryResponse, error) {
	return &MetricQueryResponse{
		Series: []MetricSeries{
			{
				Labels:     map[string]string{},
				DataPoints: []DataPoint{{Timestamp: time.Now(), Value: 65.5}},
			},
		},
	}, nil
}

func (m *mockReportMetricsRepo) CorrelateMetrics(_ context.Context, _, _, _ string, _, _ time.Time) (*ResourceCorrelation, error) {
	return &ResourceCorrelation{CorrelationCoef: 0.85}, nil
}

func (m *mockReportMetricsRepo) ExecuteQuery(_ context.Context, _ string, _ *TimeRange, _ int) (*QueryResponse, error) {
	return &QueryResponse{
		Columns: []QueryColumn{{Name: "title", Type: "String"}, {Name: "cnt", Type: "UInt64"}},
		Rows: [][]any{
			{"网络故障", float64(10)},
			{"磁盘故障", float64(5)},
		},
		TotalRows: 2,
	}, nil
}

// --- Mock SLA 仓储（用于报告模板测试） ---

// mockReportSLARepo 是 SLARepo 的模拟实现。
type mockReportSLARepo struct {
	configs []*SLAConfig
}

func newMockReportSLARepo() *mockReportSLARepo {
	return &mockReportSLARepo{
		configs: []*SLAConfig{
			{
				ConfigID:         "cfg-biz-1",
				Name:             "支付业务 SLA",
				Dimension:        SLALevelBusinessUnit,
				DimensionValue:   "payment",
				TargetPercentage: 99.95,
				Window:           "monthly",
				ExcludePlanned:   true,
			},
		},
	}
}

func (m *mockReportSLARepo) CreateConfig(_ context.Context, cfg *SLAConfig) error {
	m.configs = append(m.configs, cfg)
	return nil
}

func (m *mockReportSLARepo) GetConfig(_ context.Context, id string) (*SLAConfig, error) {
	for _, c := range m.configs {
		if c.ConfigID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (m *mockReportSLARepo) ListConfigs(_ context.Context, _ SLAListFilter) ([]*SLAConfig, int, error) {
	return m.configs, len(m.configs), nil
}

func (m *mockReportSLARepo) UpdateConfig(_ context.Context, _ *SLAConfig) error { return nil }
func (m *mockReportSLARepo) DeleteConfig(_ context.Context, _ string) error     { return nil }
func (m *mockReportSLARepo) ListConfigsByDimension(_ context.Context, dimension string) ([]*SLAConfig, error) {
	var result []*SLAConfig
	for _, c := range m.configs {
		if c.Dimension == dimension {
			result = append(result, c)
		}
	}
	return result, nil
}

// --- Mock SLA 事件仓储 ---

type mockReportSLAIncidentRepo struct{}

func (m *mockReportSLAIncidentRepo) GetDowntime(_ context.Context, _, _ string, _, _ time.Time, _ bool) (float64, int, error) {
	return 120.0, 2, nil // 模拟 120 秒停机、2 次事件
}

// --- 测试用例 ---

// TestListTemplates 验证预置模板列表的完整性。
func TestListTemplates(t *testing.T) {
	uc := NewReportTemplateUsecase(newMockReportTemplateRepo(), nil, nil, nil)
	templates := uc.ListTemplates()

	if len(templates) != 4 {
		t.Fatalf("expected 4 preset templates, got %d", len(templates))
	}

	// 验证模板 ID 集合
	expectedIDs := map[string]bool{
		TemplateMonthlyOps:    true,
		TemplateSLAReport:     true,
		TemplateIncidentTrend: true,
		TemplateAlertQuality:  true,
	}
	for _, tmpl := range templates {
		if !expectedIDs[tmpl.ID] {
			t.Errorf("unexpected template ID: %s", tmpl.ID)
		}
		if tmpl.Name == "" {
			t.Errorf("template %s has empty name", tmpl.ID)
		}
		if len(tmpl.Sections) == 0 {
			t.Errorf("template %s has no sections", tmpl.ID)
		}
	}
}

// TestGetTemplate_Found 验证获取已有模板。
func TestGetTemplate_Found(t *testing.T) {
	uc := NewReportTemplateUsecase(newMockReportTemplateRepo(), nil, nil, nil)

	tmpl, err := uc.GetTemplate(TemplateMonthlyOps)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tmpl.ID != TemplateMonthlyOps {
		t.Errorf("expected ID=%s, got %s", TemplateMonthlyOps, tmpl.ID)
	}
	if tmpl.Perspective != PerspectiveManagement {
		t.Errorf("expected perspective=management, got %s", tmpl.Perspective)
	}
}

// TestGetTemplate_NotFound 验证获取不存在的模板返回错误。
func TestGetTemplate_NotFound(t *testing.T) {
	uc := NewReportTemplateUsecase(newMockReportTemplateRepo(), nil, nil, nil)

	_, err := uc.GetTemplate("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
}

// TestGenerateMonthlyOpsReport 验证月度运营报告生成。
func TestGenerateMonthlyOpsReport(t *testing.T) {
	slaRepo := newMockReportSLARepo()
	incidentRepo := &mockReportSLAIncidentRepo{}
	metricsRepo := &mockReportMetricsRepo{}
	slaUC := NewSLAUsecase(slaRepo, incidentRepo, nil)
	templateRepo := newMockReportTemplateRepo()

	uc := NewReportTemplateUsecase(templateRepo, slaUC, metricsRepo, nil)

	report, err := uc.GenerateMonthlyOpsReport(context.Background(), 2026, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Year != 2026 || report.Month != 3 {
		t.Errorf("expected year=2026, month=3, got year=%d, month=%d", report.Year, report.Month)
	}
	if report.TemplateID != TemplateMonthlyOps {
		t.Errorf("expected template_id=%s, got %s", TemplateMonthlyOps, report.TemplateID)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

// TestGenerateSLAReport_InvalidDateRange 验证无效时间范围的错误处理。
func TestGenerateSLAReport_InvalidDateRange(t *testing.T) {
	slaRepo := newMockReportSLARepo()
	incidentRepo := &mockReportSLAIncidentRepo{}
	slaUC := NewSLAUsecase(slaRepo, incidentRepo, nil)
	uc := NewReportTemplateUsecase(newMockReportTemplateRepo(), slaUC, &mockReportMetricsRepo{}, nil)

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err := uc.GenerateSLAReport(context.Background(), start, end)
	if err == nil {
		t.Fatal("expected error for invalid date range, got nil")
	}
}

// TestGenerateIncidentTrendReport 验证事件趋势报告生成。
func TestGenerateIncidentTrendReport(t *testing.T) {
	slaRepo := newMockReportSLARepo()
	incidentRepo := &mockReportSLAIncidentRepo{}
	metricsRepo := &mockReportMetricsRepo{}
	slaUC := NewSLAUsecase(slaRepo, incidentRepo, nil)
	uc := NewReportTemplateUsecase(newMockReportTemplateRepo(), slaUC, metricsRepo, nil)

	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	report, err := uc.GenerateIncidentTrendReport(context.Background(), start, end)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.TemplateID != TemplateIncidentTrend {
		t.Errorf("expected template_id=%s, got %s", TemplateIncidentTrend, report.TemplateID)
	}
	// 应有 5 个级别的趋势数据（P0-P4）
	if len(report.SeverityTrends) != 5 {
		t.Errorf("expected 5 severity trends, got %d", len(report.SeverityTrends))
	}
}
