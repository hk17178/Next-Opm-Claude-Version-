package biz

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MetricsRepo defines the data access interface for time-series metrics (ClickHouse).
type MetricsRepo interface {
	InsertBusinessMetrics(ctx context.Context, metrics []BusinessMetric) error
	InsertResourceMetrics(ctx context.Context, metrics []ResourceMetric) error
	QueryBusinessMetrics(ctx context.Context, q MetricQueryRequest) (*MetricQueryResponse, error)
	QueryResourceMetrics(ctx context.Context, q MetricQueryRequest) (*MetricQueryResponse, error)
	CorrelateMetrics(ctx context.Context, assetID, metricA, metricB string, start, end time.Time) (*ResourceCorrelation, error)
	ExecuteQuery(ctx context.Context, query string, timeRange *TimeRange, limit int) (*QueryResponse, error)
}

// MetricsUsecase provides metrics ingestion, querying, and correlation analysis.
type MetricsUsecase struct {
	repo   MetricsRepo
	logger *zap.Logger
}

// NewMetricsUsecase creates a new metrics use case.
func NewMetricsUsecase(repo MetricsRepo, logger *zap.Logger) *MetricsUsecase {
	return &MetricsUsecase{repo: repo, logger: logger}
}

// Ingest writes metric data points to the time-series store.
// The OpenAPI contract uses a generic "metrics" field; we route to
// business_metrics or resource_metrics based on the metric name prefix.
func (uc *MetricsUsecase) Ingest(ctx context.Context, req MetricIngestRequest) error {
	if len(req.Metrics) == 0 {
		return nil
	}

	var businessMetrics []BusinessMetric
	var resourceMetrics []ResourceMetric

	for _, m := range req.Metrics {
		if isBusinessMetric(m.Name) {
			businessMetrics = append(businessMetrics, BusinessMetric{
				Timestamp:    m.Timestamp,
				MetricName:   m.Name,
				MetricValue:  m.Value,
				BusinessUnit: m.Labels["business_unit"],
				Service:      m.Labels["service"],
				Tags:         m.Labels,
			})
		} else {
			resourceMetrics = append(resourceMetrics, ResourceMetric{
				Timestamp:    m.Timestamp,
				MetricName:   m.Name,
				MetricValue:  m.Value,
				AssetID:      m.Labels["asset_id"],
				AssetType:    m.Labels["asset_type"],
				BusinessUnit: m.Labels["business_unit"],
				Region:       m.Labels["region"],
			})
		}
	}

	if len(businessMetrics) > 0 {
		if err := uc.repo.InsertBusinessMetrics(ctx, businessMetrics); err != nil {
			return fmt.Errorf("insert business metrics: %w", err)
		}
	}
	if len(resourceMetrics) > 0 {
		if err := uc.repo.InsertResourceMetrics(ctx, resourceMetrics); err != nil {
			return fmt.Errorf("insert resource metrics: %w", err)
		}
	}

	return nil
}

// Query executes a metric query against ClickHouse.
func (uc *MetricsUsecase) Query(ctx context.Context, q MetricQueryRequest) (*MetricQueryResponse, error) {
	if q.MetricName == "" {
		return nil, fmt.Errorf("metric_name is required")
	}
	if q.TimeRange.Start.IsZero() || q.TimeRange.End.IsZero() {
		return nil, fmt.Errorf("time_range start and end are required")
	}
	if q.Interval == "" {
		q.Interval = "5m"
	}
	if q.Aggregation == "" {
		q.Aggregation = "avg"
	}

	if isBusinessMetric(q.MetricName) {
		return uc.repo.QueryBusinessMetrics(ctx, q)
	}
	return uc.repo.QueryResourceMetrics(ctx, q)
}

// ExecuteQuery runs an ad-hoc analytics query (OpenAPI /query endpoint).
func (uc *MetricsUsecase) ExecuteQuery(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.Limit <= 0 {
		req.Limit = 1000
	}
	return uc.repo.ExecuteQuery(ctx, req.Query, req.TimeRange, req.Limit)
}

// Correlate computes the Pearson correlation between two resource metrics.
// FR-12-008: AI auto-insight correlation coefficient.
func (uc *MetricsUsecase) Correlate(ctx context.Context, assetID, metricA, metricB string, start, end time.Time) (*ResourceCorrelation, error) {
	if assetID == "" || metricA == "" || metricB == "" {
		return nil, fmt.Errorf("asset_id, metric_a, and metric_b are required")
	}
	return uc.repo.CorrelateMetrics(ctx, assetID, metricA, metricB, start, end)
}

// PearsonCorrelation computes the Pearson correlation coefficient for two equal-length slices.
// Returns a value in [-1, 1]. Returns 0 if fewer than 2 data points or zero variance.
// FR-12-008: correlation analysis between resource metrics.
func PearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	nf := float64(n)
	numerator := nf*sumXY - sumX*sumY
	denominator := math.Sqrt((nf*sumX2 - sumX*sumX) * (nf*sumY2 - sumY*sumY))
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

// AnomalyScore computes the z-score of a value relative to a data set.
// A value beyond +/-3 standard deviations (|z| > 3) is typically considered anomalous.
// Returns 0 if the data set has fewer than 2 elements or zero variance.
func AnomalyScore(value float64, data []float64) float64 {
	n := len(data)
	if n < 2 {
		return 0
	}

	var sum float64
	for _, v := range data {
		sum += v
	}
	mean := sum / float64(n)

	var sumSqDiff float64
	for _, v := range data {
		d := v - mean
		sumSqDiff += d * d
	}
	stddev := math.Sqrt(sumSqDiff / float64(n))
	if stddev == 0 {
		return 0
	}

	return (value - mean) / stddev
}

// isBusinessMetric determines if a metric name is a business metric by convention.
// Business metrics: tps, dau, gmv, api_calls, success_rate, payment_success_rate, etc.
// Resource metrics: cpu, memory, bandwidth, disk_io, connections, qps, etc.
func isBusinessMetric(name string) bool {
	businessPrefixes := []string{"tps", "dau", "gmv", "api_calls", "success_rate", "payment_", "transaction_", "order_"}
	lower := strings.ToLower(name)
	for _, prefix := range businessPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
