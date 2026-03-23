// Package data 实现事件管理领域的数据持久化层，基于 PostgreSQL 存储事件、时间线、变更工单和值班排班。
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// IncidentRepo 实现 biz.IncidentRepo 接口，负责事件记录的数据库持久化操作。
type IncidentRepo struct {
	db *pgxpool.Pool
}

// NewIncidentRepo 创建一个新的 IncidentRepo 实例。
func NewIncidentRepo(db *pgxpool.Pool) *IncidentRepo {
	return &IncidentRepo{db: db}
}

// NextID 生成下一个事件 ID，格式为 INC-YYYYMMDD-NNN，基于日期序列号自增。
func (r *IncidentRepo) NextID(ctx context.Context) (string, error) {
	today := time.Now().Format("20060102")
	var seq int
	err := r.db.QueryRow(ctx,
		`INSERT INTO incident_sequences (date_key, last_seq)
		 VALUES ($1, 1)
		 ON CONFLICT (date_key) DO UPDATE SET last_seq = incident_sequences.last_seq + 1
		 RETURNING last_seq`, today).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate incident id: %w", err)
	}
	return fmt.Sprintf("INC-%s-%03d", today, seq), nil
}

// Create 插入一条新的事件记录，将关联数组字段序列化为 JSON 存储。
func (r *IncidentRepo) Create(ctx context.Context, inc *biz.Incident) error {
	sourceAlerts, _ := json.Marshal(inc.SourceAlerts)
	affectedAssets, _ := json.Marshal(inc.AffectedAssets)
	tags, _ := json.Marshal(inc.Tags)
	improvements, _ := json.Marshal(inc.ImprovementItems)

	_, err := r.db.Exec(ctx,
		`INSERT INTO incidents (
			incident_id, title, severity, status, root_cause_category,
			assignee_id, assignee_name, detected_at, acknowledged_at,
			resolved_at, closed_at, source_alerts, affected_assets,
			business_unit, postmortem, improvement_items, tags,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19
		)`,
		inc.IncidentID, inc.Title, inc.Severity, inc.Status, inc.RootCauseCategory,
		inc.AssigneeID, inc.AssigneeName, inc.DetectedAt, inc.AcknowledgedAt,
		inc.ResolvedAt, inc.ClosedAt, sourceAlerts, affectedAssets,
		inc.BusinessUnit, nil, improvements, tags,
		inc.CreatedAt, inc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert incident: %w", err)
	}
	return nil
}

// GetByID 根据事件 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *IncidentRepo) GetByID(ctx context.Context, id string) (*biz.Incident, error) {
	inc := &biz.Incident{}
	var sourceAlerts, affectedAssets, tags, improvements, postmortem []byte

	err := r.db.QueryRow(ctx,
		`SELECT incident_id, title, severity, status, root_cause_category,
			assignee_id, assignee_name, detected_at, acknowledged_at,
			resolved_at, closed_at, source_alerts, affected_assets,
			business_unit, postmortem, improvement_items, tags,
			created_at, updated_at
		 FROM incidents WHERE incident_id = $1`, id).Scan(
		&inc.IncidentID, &inc.Title, &inc.Severity, &inc.Status, &inc.RootCauseCategory,
		&inc.AssigneeID, &inc.AssigneeName, &inc.DetectedAt, &inc.AcknowledgedAt,
		&inc.ResolvedAt, &inc.ClosedAt, &sourceAlerts, &affectedAssets,
		&inc.BusinessUnit, &postmortem, &improvements, &tags,
		&inc.CreatedAt, &inc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get incident: %w", err)
	}

	json.Unmarshal(sourceAlerts, &inc.SourceAlerts)
	json.Unmarshal(affectedAssets, &inc.AffectedAssets)
	json.Unmarshal(tags, &inc.Tags)
	json.Unmarshal(improvements, &inc.ImprovementItems)
	if postmortem != nil {
		var pm biz.Postmortem
		if json.Unmarshal(postmortem, &pm) == nil {
			inc.Postmortem = &pm
		}
	}

	return inc, nil
}

// Update 更新已有事件记录，自动刷新 updated_at 时间戳。
func (r *IncidentRepo) Update(ctx context.Context, inc *biz.Incident) error {
	sourceAlerts, _ := json.Marshal(inc.SourceAlerts)
	affectedAssets, _ := json.Marshal(inc.AffectedAssets)
	tags, _ := json.Marshal(inc.Tags)
	improvements, _ := json.Marshal(inc.ImprovementItems)
	var postmortemJSON []byte
	if inc.Postmortem != nil {
		postmortemJSON, _ = json.Marshal(inc.Postmortem)
	}

	inc.UpdatedAt = time.Now()

	_, err := r.db.Exec(ctx,
		`UPDATE incidents SET
			title = $2, severity = $3, status = $4, root_cause_category = $5,
			assignee_id = $6, assignee_name = $7, acknowledged_at = $8,
			resolved_at = $9, closed_at = $10, source_alerts = $11,
			affected_assets = $12, business_unit = $13, postmortem = $14,
			improvement_items = $15, tags = $16, updated_at = $17
		 WHERE incident_id = $1`,
		inc.IncidentID, inc.Title, inc.Severity, inc.Status, inc.RootCauseCategory,
		inc.AssigneeID, inc.AssigneeName, inc.AcknowledgedAt,
		inc.ResolvedAt, inc.ClosedAt, sourceAlerts,
		affectedAssets, inc.BusinessUnit, postmortemJSON,
		improvements, tags, inc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update incident: %w", err)
	}
	return nil
}

// List 根据过滤条件和分页参数查询事件列表，支持动态 WHERE 子句构建。
func (r *IncidentRepo) List(ctx context.Context, f biz.ListFilter) ([]*biz.Incident, int64, error) {
	// 动态构建 WHERE 子句
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if f.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*f.Status))
		argIdx++
	}
	if f.Severity != nil {
		where += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, *f.Severity)
		argIdx++
	}
	if f.AssigneeID != nil {
		where += fmt.Sprintf(" AND assignee_id = $%d", argIdx)
		args = append(args, *f.AssigneeID)
		argIdx++
	}
	if f.BusinessUnit != nil {
		where += fmt.Sprintf(" AND business_unit = $%d", argIdx)
		args = append(args, *f.BusinessUnit)
		argIdx++
	}

	// 查询符合条件的总记录数
	var total int64
	countQuery := "SELECT COUNT(*) FROM incidents " + where
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count incidents: %w", err)
	}

	// 校验排序字段白名单，防止 SQL 注入
	sortCol := "created_at"
	if f.SortBy == "detected_at" || f.SortBy == "severity" || f.SortBy == "status" || f.SortBy == "updated_at" {
		sortCol = f.SortBy
	}
	sortOrder := "DESC"
	if f.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	offset := (f.Page - 1) * f.PageSize
	query := fmt.Sprintf(
		`SELECT incident_id, title, severity, status, root_cause_category,
			assignee_id, assignee_name, detected_at, acknowledged_at,
			resolved_at, closed_at, source_alerts, affected_assets,
			business_unit, postmortem, improvement_items, tags,
			created_at, updated_at
		 FROM incidents %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, sortCol, sortOrder, argIdx, argIdx+1,
	)
	args = append(args, f.PageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []*biz.Incident
	for rows.Next() {
		inc := &biz.Incident{}
		var sourceAlerts, affectedAssets, tags, improvements, postmortem []byte
		if err := rows.Scan(
			&inc.IncidentID, &inc.Title, &inc.Severity, &inc.Status, &inc.RootCauseCategory,
			&inc.AssigneeID, &inc.AssigneeName, &inc.DetectedAt, &inc.AcknowledgedAt,
			&inc.ResolvedAt, &inc.ClosedAt, &sourceAlerts, &affectedAssets,
			&inc.BusinessUnit, &postmortem, &improvements, &tags,
			&inc.CreatedAt, &inc.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan incident: %w", err)
		}
		json.Unmarshal(sourceAlerts, &inc.SourceAlerts)
		json.Unmarshal(affectedAssets, &inc.AffectedAssets)
		json.Unmarshal(tags, &inc.Tags)
		json.Unmarshal(improvements, &inc.ImprovementItems)
		if postmortem != nil {
			var pm biz.Postmortem
			if json.Unmarshal(postmortem, &pm) == nil {
				inc.Postmortem = &pm
			}
		}
		incidents = append(incidents, inc)
	}

	return incidents, total, nil
}

// Delete 根据事件 ID 删除一条记录。
func (r *IncidentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM incidents WHERE incident_id = $1", id)
	return err
}

// SetMTTR 将指定事件的 MTTR（平均修复时间，单位：秒）持久化到 incidents.mttr_seconds 列。
//
// 触发时机：事件状态流转到 StatusClosed 时，由 IncidentUsecase.UpdateStatus 自动调用。
//
// 计算公式：mttr_seconds = resolved_at - detected_at（秒）。
// 若事件在关闭前未经过 resolved 状态（resolved_at 为 NULL），则不调用此方法，
// mttr_seconds 列保持 NULL。
//
// 参数：
//   - ctx: 请求上下文。
//   - incidentID: 事件唯一标识，如 INC-20250101-001。
//   - mttrSeconds: 已计算好的 MTTR 秒数，由业务层传入。
//
// 返回值：
//   - error: SQL 执行失败时返回错误。
func (r *IncidentRepo) SetMTTR(ctx context.Context, incidentID string, mttrSeconds int64) error {
	now := time.Now()
	return SetMTTROnDB(ctx, r.db, incidentID, mttrSeconds, now)
}
