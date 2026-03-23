// Package data 实现告警服务的数据持久化层，基于 PostgreSQL (pgx) 提供
// 告警实例和告警规则的 CRUD 操作。
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opsnexus/svc-alert/internal/biz"
)

// AlertRepo 实现 biz.AlertRepository 接口，提供告警实例的 PostgreSQL 持久化。
type AlertRepo struct {
	pool *pgxpool.Pool
}

// NewAlertRepo 创建告警仓储实例，pool 为 PostgreSQL 连接池。
func NewAlertRepo(pool *pgxpool.Pool) *AlertRepo {
	return &AlertRepo{pool: pool}
}

// Create 将告警实例持久化到 alerts 表。
func (r *AlertRepo) Create(alert *biz.Alert) error {
	tagsJSON, _ := json.Marshal(alert.Tags)

	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO alerts (alert_id, rule_id, severity, status, title, description,
		 source_host, source_service, source_asset_id, message, metric_value,
		 threshold_value, fingerprint, layer, ironclad, suppressed, suppressed_by,
		 triggered_at, acknowledged_at, resolved_at, incident_id, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		alert.AlertID, alert.RuleID, alert.Severity, alert.Status, alert.Title,
		alert.Description, alert.SourceHost, alert.SourceService, nilIfEmpty(alert.SourceAssetID),
		alert.Message, alert.MetricValue, alert.ThresholdValue, alert.Fingerprint,
		alert.Layer, alert.Ironclad, alert.Suppressed, nilIfEmpty(alert.SuppressedBy),
		alert.TriggeredAt, alert.AcknowledgedAt, alert.ResolvedAt,
		nilIfEmpty(alert.IncidentID), tagsJSON,
	)
	return err
}

// GetByID 按告警 ID 查询单条告警记录。
func (r *AlertRepo) GetByID(id string) (*biz.Alert, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT alert_id, rule_id, severity, status, title, description,
		 source_host, source_service, source_asset_id, message, metric_value,
		 threshold_value, fingerprint, layer, ironclad, suppressed, suppressed_by,
		 triggered_at, acknowledged_at, resolved_at, incident_id, tags
		 FROM alerts WHERE alert_id=$1`, id)

	return r.scanAlert(row)
}

// UpdateStatus 更新告警状态（确认/解决）及对应时间戳。
func (r *AlertRepo) UpdateStatus(id string, status biz.AlertStatus, acknowledgedAt, resolvedAt *time.Time) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE alerts SET status=$2, acknowledged_at=$3, resolved_at=$4 WHERE alert_id=$1`,
		id, status, acknowledgedAt, resolvedAt,
	)
	return err
}

// GetByFingerprint 按指纹查询最近一条处于 firing 状态的告警，用于去重时更新抑制信息。
func (r *AlertRepo) GetByFingerprint(fingerprint string) (*biz.Alert, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT alert_id, rule_id, severity, status, title, description,
		 source_host, source_service, source_asset_id, message, metric_value,
		 threshold_value, fingerprint, layer, ironclad, suppressed, suppressed_by,
		 triggered_at, acknowledged_at, resolved_at, incident_id, tags
		 FROM alerts WHERE fingerprint=$1 AND status='firing'
		 ORDER BY triggered_at DESC LIMIT 1`, fingerprint)

	return r.scanAlert(row)
}

// GetActiveByRuleID 查询指定规则下所有处于 firing 状态的活跃告警。
func (r *AlertRepo) GetActiveByRuleID(ruleID string) ([]*biz.Alert, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT alert_id, rule_id, severity, status, title, description,
		 source_host, source_service, source_asset_id, message, metric_value,
		 threshold_value, fingerprint, layer, ironclad, suppressed, suppressed_by,
		 triggered_at, acknowledged_at, resolved_at, incident_id, tags
		 FROM alerts WHERE rule_id=$1 AND status='firing'
		 ORDER BY triggered_at DESC`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// List 分页查询告警列表，支持按状态和严重等级过滤。
// 使用 keyset 分页（基于 alert_id），多取一条判断是否有下一页。
func (r *AlertRepo) List(status *biz.AlertStatus, severity *biz.Severity, pageSize int, pageToken string) ([]*biz.Alert, string, error) {
	query := `SELECT alert_id, rule_id, severity, status, title, description,
		source_host, source_service, source_asset_id, message, metric_value,
		threshold_value, fingerprint, layer, ironclad, suppressed, suppressed_by,
		triggered_at, acknowledged_at, resolved_at, incident_id, tags
		FROM alerts WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *status)
		argIdx++
	}
	if severity != nil {
		query += fmt.Sprintf(` AND severity = $%d`, argIdx)
		args = append(args, *severity)
		argIdx++
	}
	if pageToken != "" {
		query += fmt.Sprintf(` AND alert_id < $%d`, argIdx)
		args = append(args, pageToken)
		argIdx++
	}

	query += ` ORDER BY triggered_at DESC`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, pageSize+1)

	rows, err := r.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	alerts, err := r.scanAlerts(rows)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if len(alerts) > pageSize {
		nextToken = alerts[pageSize].AlertID
		alerts = alerts[:pageSize]
	}

	return alerts, nextToken, nil
}

// IncrementSuppression 标记告警为已抑制并记录抑制来源（如 "convergence"）。
func (r *AlertRepo) IncrementSuppression(id string, suppressedBy string) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE alerts SET suppressed = true, suppressed_by = $2 WHERE alert_id=$1`, id, suppressedBy)
	return err
}

// NextAlertID 从数据库序列 alert_daily_seq 生成下一个告警 ID，格式为 ALT-YYYYMMDD-NNN。
func (r *AlertRepo) NextAlertID() (string, error) {
	var seq int64
	err := r.pool.QueryRow(context.Background(), `SELECT nextval('alert_daily_seq')`).Scan(&seq)
	if err != nil {
		return "", err
	}
	today := time.Now().Format("20060102")
	return fmt.Sprintf("ALT-%s-%03d", today, seq), nil
}

// scanAlert 从单行查询结果扫描为 biz.Alert 结构体。
func (r *AlertRepo) scanAlert(row pgx.Row) (*biz.Alert, error) {
	var alert biz.Alert
	var tagsJSON []byte

	err := row.Scan(
		&alert.AlertID, &alert.RuleID, &alert.Severity, &alert.Status,
		&alert.Title, &alert.Description, &alert.SourceHost, &alert.SourceService,
		&alert.SourceAssetID, &alert.Message, &alert.MetricValue, &alert.ThresholdValue,
		&alert.Fingerprint, &alert.Layer, &alert.Ironclad, &alert.Suppressed,
		&alert.SuppressedBy, &alert.TriggeredAt, &alert.AcknowledgedAt,
		&alert.ResolvedAt, &alert.IncidentID, &tagsJSON,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(tagsJSON, &alert.Tags)
	return &alert, nil
}

// scanAlerts 从多行查询结果扫描为 biz.Alert 切片。
func (r *AlertRepo) scanAlerts(rows pgx.Rows) ([]*biz.Alert, error) {
	var alerts []*biz.Alert
	for rows.Next() {
		var alert biz.Alert
		var tagsJSON []byte

		err := rows.Scan(
			&alert.AlertID, &alert.RuleID, &alert.Severity, &alert.Status,
			&alert.Title, &alert.Description, &alert.SourceHost, &alert.SourceService,
			&alert.SourceAssetID, &alert.Message, &alert.MetricValue, &alert.ThresholdValue,
			&alert.Fingerprint, &alert.Layer, &alert.Ironclad, &alert.Suppressed,
			&alert.SuppressedBy, &alert.TriggeredAt, &alert.AcknowledgedAt,
			&alert.ResolvedAt, &alert.IncidentID, &tagsJSON,
		)
		if err != nil {
			return nil, err
		}

		_ = json.Unmarshal(tagsJSON, &alert.Tags)
		alerts = append(alerts, &alert)
	}
	return alerts, rows.Err()
}

// nilIfEmpty 将空字符串转为 nil，用于 PostgreSQL 可空字段的插入。
func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
