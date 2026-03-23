package biz

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

// --- Mock MetricsRepo ---

type mockMetricsRepo struct {
	businessMetrics []BusinessMetric
	resourceMetrics []ResourceMetric
	correlation     *ResourceCorrelation
	correlateErr    error
}

func (m *mockMetricsRepo) InsertBusinessMetrics(_ context.Context, metrics []BusinessMetric) error {
	m.businessMetrics = append(m.businessMetrics, metrics...)
	return nil
}

func (m *mockMetricsRepo) InsertResourceMetrics(_ context.Context, metrics []ResourceMetric) error {
	m.resourceMetrics = append(m.resourceMetrics, metrics...)
	return nil
}

func (m *mockMetricsRepo) QueryBusinessMetrics(_ context.Context, _ MetricQueryRequest) (*MetricQueryResponse, error) {
	return &MetricQueryResponse{
		Series: []MetricSeries{
			{
				Labels:     map[string]string{"business_unit": "payment"},
				DataPoints: []DataPoint{{Timestamp: time.Now(), Value: 1234.5}},
			},
		},
	}, nil
}

func (m *mockMetricsRepo) QueryResourceMetrics(_ context.Context, _ MetricQueryRequest) (*MetricQueryResponse, error) {
	return &MetricQueryResponse{
		Series: []MetricSeries{
			{
				Labels:     map[string]string{"asset_id": "srv-001"},
				DataPoints: []DataPoint{{Timestamp: time.Now(), Value: 78.5}},
			},
		},
	}, nil
}

func (m *mockMetricsRepo) CorrelateMetrics(_ context.Context, assetID, metricA, metricB string, _, _ time.Time) (*ResourceCorrelation, error) {
	if m.correlateErr != nil {
		return nil, m.correlateErr
	}
	if m.correlation != nil {
		return m.correlation, nil
	}
	return &ResourceCorrelation{
		MetricA:         metricA,
		MetricB:         metricB,
		AssetID:         assetID,
		CorrelationCoef: 0.87,
		SampleCount:     100,
	}, nil
}

func (m *mockMetricsRepo) ExecuteQuery(_ context.Context, query string, _ *TimeRange, _ int) (*QueryResponse, error) {
	return &QueryResponse{
		Columns:   []QueryColumn{{Name: "count", Type: "UInt64"}},
		Rows:      [][]any{{42}},
		TotalRows: 1,
	}, nil
}

// --- Ingest and Query Tests ---

func TestMetricsIngest_RoutesToCorrectTable(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	req := MetricIngestRequest{
		Metrics: []MetricIngestPoint{
			{Name: "tps", Value: 500, Timestamp: time.Now(), Labels: map[string]string{"business_unit": "payment", "service": "checkout"}},
			{Name: "cpu", Value: 85.3, Timestamp: time.Now(), Labels: map[string]string{"asset_id": "srv-001", "asset_type": "server"}},
			{Name: "transaction_count", Value: 1000, Timestamp: time.Now(), Labels: map[string]string{"business_unit": "orders"}},
			{Name: "memory", Value: 72.1, Timestamp: time.Now(), Labels: map[string]string{"asset_id": "srv-002"}},
		},
	}

	err := uc.Ingest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(repo.businessMetrics) != 2 {
		t.Errorf("expected 2 business metrics (tps, transaction_count), got %d", len(repo.businessMetrics))
	}
	if len(repo.resourceMetrics) != 2 {
		t.Errorf("expected 2 resource metrics (cpu, memory), got %d", len(repo.resourceMetrics))
	}
}

func TestMetricsIngest_EmptyRequest(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	err := uc.Ingest(context.Background(), MetricIngestRequest{})
	if err != nil {
		t.Errorf("expected no error for empty ingest, got %v", err)
	}
}

func TestMetricsQuery_BusinessMetric(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	q := MetricQueryRequest{
		MetricName: "tps",
		TimeRange:  TimeRange{Start: time.Now().Add(-1 * time.Hour), End: time.Now()},
	}

	resp, err := uc.Query(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Series) == 0 {
		t.Error("expected at least one series")
	}
	if resp.Series[0].Labels["business_unit"] != "payment" {
		t.Errorf("expected business_unit=payment label")
	}
}

func TestMetricsQuery_ResourceMetric(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	q := MetricQueryRequest{
		MetricName: "cpu",
		TimeRange:  TimeRange{Start: time.Now().Add(-1 * time.Hour), End: time.Now()},
	}

	resp, err := uc.Query(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Series) == 0 {
		t.Error("expected at least one series")
	}
	if resp.Series[0].Labels["asset_id"] != "srv-001" {
		t.Errorf("expected asset_id=srv-001 label")
	}
}

func TestMetricsQuery_ValidationErrors(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	_, err := uc.Query(context.Background(), MetricQueryRequest{
		TimeRange: TimeRange{Start: time.Now().Add(-1 * time.Hour), End: time.Now()},
	})
	if err == nil {
		t.Error("expected error for missing metric_name")
	}

	_, err = uc.Query(context.Background(), MetricQueryRequest{
		MetricName: "cpu",
	})
	if err == nil {
		t.Error("expected error for missing time_range")
	}
}

func TestMetricsCorrelate_Success(t *testing.T) {
	repo := &mockMetricsRepo{
		correlation: &ResourceCorrelation{
			MetricA:         "cpu",
			MetricB:         "tps",
			AssetID:         "srv-001",
			CorrelationCoef: 0.92,
			SampleCount:     288,
		},
	}
	uc := NewMetricsUsecase(repo, nil)

	corr, err := uc.Correlate(context.Background(), "srv-001", "cpu", "tps",
		time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if corr.CorrelationCoef != 0.92 {
		t.Errorf("expected correlation=0.92, got %f", corr.CorrelationCoef)
	}
	if corr.SampleCount != 288 {
		t.Errorf("expected sample_count=288, got %d", corr.SampleCount)
	}
}

func TestMetricsCorrelate_ValidationErrors(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	tests := []struct {
		name    string
		assetID string
		metricA string
		metricB string
	}{
		{"missing asset_id", "", "cpu", "memory"},
		{"missing metric_a", "srv-001", "", "memory"},
		{"missing metric_b", "srv-001", "cpu", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Correlate(context.Background(), tt.assetID, tt.metricA, tt.metricB, start, end)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestMetricsCorrelate_RepoError(t *testing.T) {
	repo := &mockMetricsRepo{correlateErr: fmt.Errorf("clickhouse connection failed")}
	uc := NewMetricsUsecase(repo, nil)

	_, err := uc.Correlate(context.Background(), "srv-001", "cpu", "memory",
		time.Now().Add(-1*time.Hour), time.Now())
	if err == nil {
		t.Error("expected error from repo")
	}
}

func TestMetricsExecuteQuery(t *testing.T) {
	repo := &mockMetricsRepo{}
	uc := NewMetricsUsecase(repo, nil)

	resp, err := uc.ExecuteQuery(context.Background(), QueryRequest{
		Query: "SELECT count() FROM business_metrics",
		Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalRows != 1 {
		t.Errorf("expected 1 row, got %d", resp.TotalRows)
	}
}

func TestIsBusinessMetric(t *testing.T) {
	businessNames := []string{"tps", "dau", "gmv", "api_calls", "success_rate", "payment_success", "transaction_count", "order_total"}
	for _, name := range businessNames {
		if !isBusinessMetric(name) {
			t.Errorf("expected %q to be classified as business metric", name)
		}
	}

	resourceNames := []string{"cpu", "memory", "bandwidth", "disk_io", "connections", "qps", "latency"}
	for _, name := range resourceNames {
		if isBusinessMetric(name) {
			t.Errorf("expected %q to be classified as resource metric", name)
		}
	}
}

// --- Pearson Correlation Tests ---

// TestPearsonCorrelation verifies the Pearson correlation coefficient calculation.
// FR-12-008: cross-dimension correlation analysis.
func TestPearsonCorrelation(t *testing.T) {
	tests := []struct {
		name      string
		x         []float64
		y         []float64
		wantLow   float64
		wantHigh  float64
	}{
		{
			name:     "perfect positive correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{2, 4, 6, 8, 10},
			wantLow:  0.999,
			wantHigh: 1.001,
		},
		{
			name:     "perfect negative correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{10, 8, 6, 4, 2},
			wantLow:  -1.001,
			wantHigh: -0.999,
		},
		{
			name:     "no correlation (orthogonal)",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{2, 1, 4, 3, 5},
			wantLow:  -0.5,
			wantHigh: 0.9,
		},
		{
			name:     "strong positive correlation (cpu vs latency)",
			x:        []float64{20, 35, 50, 65, 80, 95},
			y:        []float64{10, 18, 30, 42, 55, 70},
			wantLow:  0.99,
			wantHigh: 1.001,
		},
		{
			name:     "constant x (zero variance) returns 0",
			x:        []float64{5, 5, 5, 5},
			y:        []float64{1, 2, 3, 4},
			wantLow:  -0.001,
			wantHigh: 0.001,
		},
		{
			name:     "constant y (zero variance) returns 0",
			x:        []float64{1, 2, 3, 4},
			y:        []float64{7, 7, 7, 7},
			wantLow:  -0.001,
			wantHigh: 0.001,
		},
		{
			name:     "single point returns 0",
			x:        []float64{42},
			y:        []float64{99},
			wantLow:  -0.001,
			wantHigh: 0.001,
		},
		{
			name:     "empty slices return 0",
			x:        []float64{},
			y:        []float64{},
			wantLow:  -0.001,
			wantHigh: 0.001,
		},
		{
			name:     "mismatched lengths return 0",
			x:        []float64{1, 2, 3},
			y:        []float64{4, 5},
			wantLow:  -0.001,
			wantHigh: 0.001,
		},
		{
			name:     "real-world: moderate negative (memory vs free_mem)",
			x:        []float64{60, 65, 70, 75, 80, 85, 90},
			y:        []float64{40, 35, 30, 25, 20, 15, 10},
			wantLow:  -1.001,
			wantHigh: -0.999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PearsonCorrelation(tt.x, tt.y)
			if got < tt.wantLow || got > tt.wantHigh {
				t.Errorf("PearsonCorrelation() = %f, want in [%f, %f]", got, tt.wantLow, tt.wantHigh)
			}
		})
	}
}

// --- Anomaly Score Tests ---

// TestAnomalyScore verifies z-score based anomaly detection.
// A value beyond mean +/- 3 standard deviations (|z| > 3) is anomalous.
func TestAnomalyScore(t *testing.T) {
	// Normal CPU data: mean ~50, stddev ~15.81
	normalCPU := []float64{30, 35, 40, 45, 50, 55, 60, 65, 70}

	t.Run("normal value has small z-score", func(t *testing.T) {
		z := AnomalyScore(50, normalCPU)
		if math.Abs(z) > 1.0 {
			t.Errorf("expected |z| < 1 for mean value, got %f", z)
		}
	})

	t.Run("mild deviation has moderate z-score", func(t *testing.T) {
		z := AnomalyScore(70, normalCPU)
		if math.Abs(z) < 1.0 || math.Abs(z) > 2.0 {
			t.Errorf("expected 1 < |z| < 2 for mild deviation, got %f", z)
		}
	})

	t.Run("anomalous value exceeds 3 sigma", func(t *testing.T) {
		z := AnomalyScore(110, normalCPU)
		if math.Abs(z) <= 3.0 {
			t.Errorf("expected |z| > 3 for anomalous value 110, got %f", z)
		}
	})

	t.Run("negative anomaly exceeds -3 sigma", func(t *testing.T) {
		z := AnomalyScore(-10, normalCPU)
		if z >= -3.0 {
			t.Errorf("expected z < -3 for anomalous value -10, got %f", z)
		}
	})

	t.Run("value at mean has z-score near 0", func(t *testing.T) {
		data := []float64{100, 100, 100, 100, 100}
		z := AnomalyScore(100, data)
		if z != 0 {
			t.Errorf("expected z=0 for constant data, got %f", z)
		}
	})

	t.Run("constant data returns 0 for any value", func(t *testing.T) {
		data := []float64{42, 42, 42, 42}
		z := AnomalyScore(99, data)
		if z != 0 {
			t.Errorf("expected z=0 for zero-variance data, got %f", z)
		}
	})

	t.Run("single data point returns 0", func(t *testing.T) {
		z := AnomalyScore(50, []float64{50})
		if z != 0 {
			t.Errorf("expected z=0 for single data point, got %f", z)
		}
	})

	t.Run("empty data returns 0", func(t *testing.T) {
		z := AnomalyScore(50, []float64{})
		if z != 0 {
			t.Errorf("expected z=0 for empty data, got %f", z)
		}
	})

	t.Run("real-world: latency spike detection", func(t *testing.T) {
		// Normal latency ~20ms, sudden spike to 200ms
		latencies := []float64{18, 20, 22, 19, 21, 20, 18, 22, 19, 21}
		spike := 200.0
		z := AnomalyScore(spike, latencies)
		if math.Abs(z) <= 3.0 {
			t.Errorf("expected latency spike to be anomalous (|z| > 3), got %f", z)
		}
	})

	t.Run("symmetric: positive and negative deviations", func(t *testing.T) {
		data := []float64{0, 0, 0, 0, 0}
		// stddev = 0, so these should return 0
		data = []float64{-10, -5, 0, 5, 10}
		zHigh := AnomalyScore(30, data)
		zLow := AnomalyScore(-30, data)
		// Symmetric data -> symmetric z-scores
		if math.Abs(math.Abs(zHigh)-math.Abs(zLow)) > 0.001 {
			t.Errorf("expected symmetric z-scores, got high=%f low=%f", zHigh, zLow)
		}
		if zHigh <= 0 {
			t.Errorf("expected positive z for value above mean, got %f", zHigh)
		}
		if zLow >= 0 {
			t.Errorf("expected negative z for value below mean, got %f", zLow)
		}
	})
}
