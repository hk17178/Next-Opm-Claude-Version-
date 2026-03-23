package data

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/opsnexus/svc-analytics/internal/biz"
)

// MetricsRepoImpl implements biz.MetricsRepo using ClickHouse.
type MetricsRepoImpl struct {
	conn driver.Conn
}

// NewMetricsRepo creates a new metrics repository.
func NewMetricsRepo(conn driver.Conn) *MetricsRepoImpl {
	return &MetricsRepoImpl{conn: conn}
}

func (r *MetricsRepoImpl) InsertBusinessMetrics(ctx context.Context, metrics []biz.BusinessMetric) error {
	batch, err := r.conn.PrepareBatch(ctx, `
		INSERT INTO business_metrics (timestamp, metric_name, metric_value, business_unit, service, tags)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, m := range metrics {
		if err := batch.Append(m.Timestamp, m.MetricName, m.MetricValue, m.BusinessUnit, m.Service, m.Tags); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *MetricsRepoImpl) InsertResourceMetrics(ctx context.Context, metrics []biz.ResourceMetric) error {
	batch, err := r.conn.PrepareBatch(ctx, `
		INSERT INTO resource_metrics (timestamp, metric_name, metric_value, asset_id, asset_type, business_unit, region)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, m := range metrics {
		if err := batch.Append(m.Timestamp, m.MetricName, m.MetricValue, m.AssetID, m.AssetType, m.BusinessUnit, m.Region); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *MetricsRepoImpl) QueryBusinessMetrics(ctx context.Context, q biz.MetricQueryRequest) (*biz.MetricQueryResponse, error) {
	aggFunc := toClickHouseAgg(q.Aggregation)
	interval := toClickHouseInterval(q.Interval)

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL %s) AS ts,
			business_unit,
			service,
			%s(metric_value) AS agg_value
		FROM business_metrics
		WHERE timestamp >= ? AND timestamp < ?
		  AND metric_name = ?`, interval, aggFunc)

	args := []any{q.TimeRange.Start, q.TimeRange.End, q.MetricName}

	if bu, ok := q.Filters["business_unit"]; ok {
		query += ` AND business_unit = ?`
		args = append(args, bu)
	}
	if svc, ok := q.Filters["service"]; ok {
		query += ` AND service = ?`
		args = append(args, svc)
	}

	query += ` GROUP BY ts, business_unit, service ORDER BY ts`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query business metrics: %w", err)
	}
	defer rows.Close()

	seriesMap := make(map[string]*biz.MetricSeries)
	for rows.Next() {
		var ts time.Time
		var bu, svc string
		var val float64
		if err := rows.Scan(&ts, &bu, &svc, &val); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		key := bu + "|" + svc
		s, ok := seriesMap[key]
		if !ok {
			s = &biz.MetricSeries{
				Labels: map[string]string{"business_unit": bu, "service": svc},
			}
			seriesMap[key] = s
		}
		s.DataPoints = append(s.DataPoints, biz.DataPoint{Timestamp: ts, Value: val})
	}

	resp := &biz.MetricQueryResponse{}
	for _, s := range seriesMap {
		resp.Series = append(resp.Series, *s)
	}
	return resp, nil
}

func (r *MetricsRepoImpl) QueryResourceMetrics(ctx context.Context, q biz.MetricQueryRequest) (*biz.MetricQueryResponse, error) {
	aggFunc := toClickHouseAgg(q.Aggregation)
	interval := toClickHouseInterval(q.Interval)

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL %s) AS ts,
			asset_id,
			asset_type,
			%s(metric_value) AS agg_value
		FROM resource_metrics
		WHERE timestamp >= ? AND timestamp < ?
		  AND metric_name = ?`, interval, aggFunc)

	args := []any{q.TimeRange.Start, q.TimeRange.End, q.MetricName}

	if assetID, ok := q.Filters["asset_id"]; ok {
		query += ` AND asset_id = ?`
		args = append(args, assetID)
	}
	if region, ok := q.Filters["region"]; ok {
		query += ` AND region = ?`
		args = append(args, region)
	}

	query += ` GROUP BY ts, asset_id, asset_type ORDER BY ts`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resource metrics: %w", err)
	}
	defer rows.Close()

	seriesMap := make(map[string]*biz.MetricSeries)
	for rows.Next() {
		var ts time.Time
		var assetID, assetType string
		var val float64
		if err := rows.Scan(&ts, &assetID, &assetType, &val); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		key := assetID
		s, ok := seriesMap[key]
		if !ok {
			s = &biz.MetricSeries{
				Labels: map[string]string{"asset_id": assetID, "asset_type": assetType},
			}
			seriesMap[key] = s
		}
		s.DataPoints = append(s.DataPoints, biz.DataPoint{Timestamp: ts, Value: val})
	}

	resp := &biz.MetricQueryResponse{}
	for _, s := range seriesMap {
		resp.Series = append(resp.Series, *s)
	}
	return resp, nil
}

func (r *MetricsRepoImpl) CorrelateMetrics(ctx context.Context, assetID, metricA, metricB string, start, end time.Time) (*biz.ResourceCorrelation, error) {
	query := `
		SELECT
			corr(a.metric_value, b.metric_value) AS correlation,
			count() AS sample_count
		FROM (
			SELECT toStartOfInterval(timestamp, INTERVAL 5 minute) AS ts, avg(metric_value) AS metric_value
			FROM resource_metrics
			WHERE asset_id = ? AND metric_name = ? AND timestamp >= ? AND timestamp < ?
			GROUP BY ts
		) a
		INNER JOIN (
			SELECT toStartOfInterval(timestamp, INTERVAL 5 minute) AS ts, avg(metric_value) AS metric_value
			FROM resource_metrics
			WHERE asset_id = ? AND metric_name = ? AND timestamp >= ? AND timestamp < ?
			GROUP BY ts
		) b ON a.ts = b.ts`

	var corr float64
	var sampleCount uint64
	err := r.conn.QueryRow(ctx, query,
		assetID, metricA, start, end,
		assetID, metricB, start, end,
	).Scan(&corr, &sampleCount)
	if err != nil {
		return nil, fmt.Errorf("correlate: %w", err)
	}

	if math.IsNaN(corr) {
		corr = 0
	}

	return &biz.ResourceCorrelation{
		MetricA:         metricA,
		MetricB:         metricB,
		AssetID:         assetID,
		CorrelationCoef: corr,
		SampleCount:     int(sampleCount),
	}, nil
}

func (r *MetricsRepoImpl) ExecuteQuery(ctx context.Context, query string, timeRange *biz.TimeRange, limit int) (*biz.QueryResponse, error) {
	startTime := time.Now()

	if limit <= 0 {
		limit = 1000
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := r.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	colTypes := rows.ColumnTypes()
	columns := make([]biz.QueryColumn, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = biz.QueryColumn{
			Name: ct.Name(),
			Type: ct.DatabaseTypeName(),
		}
	}

	var resultRows [][]any
	for rows.Next() {
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		resultRows = append(resultRows, vals)
	}

	return &biz.QueryResponse{
		Columns:         columns,
		Rows:            resultRows,
		TotalRows:       len(resultRows),
		ExecutionTimeMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// SLAIncidentRepoImpl implements biz.SLAIncidentRepo using ClickHouse.
type SLAIncidentRepoImpl struct {
	conn driver.Conn
}

// NewSLAIncidentRepo creates a new SLA incident repository.
func NewSLAIncidentRepo(conn driver.Conn) *SLAIncidentRepoImpl {
	return &SLAIncidentRepoImpl{conn: conn}
}

// GetDowntime queries the sla_incidents table for total unplanned downtime.
func (r *SLAIncidentRepoImpl) GetDowntime(ctx context.Context, dimension, dimensionValue string, start, end time.Time, excludePlanned bool) (float64, int, error) {
	dimColumn, err := dimensionToColumn(dimension)
	if err != nil {
		return 0, 0, err
	}

	query := fmt.Sprintf(`
		SELECT
			sum(downtime_seconds) AS total_downtime,
			count() AS incident_count
		FROM sla_incidents
		WHERE %s = ?
		  AND detected_at >= ? AND detected_at < ?`, dimColumn)

	args := []any{dimensionValue, start, end}

	if excludePlanned {
		query += ` AND is_planned = 0`
	}

	var totalDowntime float64
	var incidentCount uint64
	err = r.conn.QueryRow(ctx, query, args...).Scan(&totalDowntime, &incidentCount)
	if err != nil {
		return 0, 0, fmt.Errorf("get downtime: %w", err)
	}

	return totalDowntime, int(incidentCount), nil
}

func dimensionToColumn(dimension string) (string, error) {
	switch dimension {
	case "infra_layer":
		return "infra_layer", nil
	case "business_unit":
		return "business_unit", nil
	case "asset_group":
		return "asset_group", nil
	case "asset_grade":
		return "asset_grade", nil
	case "region":
		return "region", nil
	case "asset":
		return "incident_id", nil // single-asset SLA uses incident_id match
	case "global":
		return "1", nil // match all — caller passes "1" as dimensionValue
	default:
		return "", fmt.Errorf("unknown dimension: %s", dimension)
	}
}

func toClickHouseAgg(agg string) string {
	switch strings.ToLower(agg) {
	case "max":
		return "max"
	case "min":
		return "min"
	case "sum":
		return "sum"
	case "count":
		return "count"
	case "p50":
		return "quantile(0.50)"
	case "p90":
		return "quantile(0.90)"
	case "p95":
		return "quantile(0.95)"
	case "p99":
		return "quantile(0.99)"
	default:
		return "avg"
	}
}

func toClickHouseInterval(interval string) string {
	switch interval {
	case "1m":
		return "1 minute"
	case "5m":
		return "5 minute"
	case "1h":
		return "1 hour"
	case "1d":
		return "1 day"
	default:
		return "5 minute"
	}
}
