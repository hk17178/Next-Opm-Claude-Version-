// Package data 实现变更管理领域的数据持久化层，基于 PostgreSQL 存储变更单和审批记录。
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-change/internal/biz"
)

// ChangeRepo 实现 biz.ChangeRepo 接口，负责变更单的数据库持久化操作。
type ChangeRepo struct {
	db *pgxpool.Pool
}

// NewChangeRepo 创建一个新的 ChangeRepo 实例。
func NewChangeRepo(db *pgxpool.Pool) *ChangeRepo {
	return &ChangeRepo{db: db}
}

// NextID 生成下一个变更单 ID，格式为 CHG-YYYYMMDD-NNN，基于日期序列号自增。
func (r *ChangeRepo) NextID(ctx context.Context) (string, error) {
	today := time.Now().Format("20060102")
	var seq int
	err := r.db.QueryRow(ctx,
		`INSERT INTO change_sequences (date_key, last_seq)
		 VALUES ($1, 1)
		 ON CONFLICT (date_key) DO UPDATE SET last_seq = change_sequences.last_seq + 1
		 RETURNING last_seq`, today).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate change id: %w", err)
	}
	return fmt.Sprintf("CHG-%s-%03d", today, seq), nil
}

// Create 插入一条新的变更单记录。
func (r *ChangeRepo) Create(ctx context.Context, ticket *biz.ChangeTicket) error {
	approvers, _ := json.Marshal(ticket.Approvers)
	affectedAssets, _ := json.Marshal(ticket.AffectedAssets)
	relatedChangeIDs, _ := json.Marshal(ticket.RelatedChangeIDs)

	_, err := r.db.Exec(ctx,
		`INSERT INTO change_tickets (
			id, title, type, risk_level, status,
			requester, approvers, executor_id, affected_assets,
			rollback_plan, scheduled_start, scheduled_end,
			actual_start, actual_end, description,
			ai_risk_summary, related_change_ids, maintenance_id,
			cancel_reason, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20, $21
		)`,
		ticket.ID, ticket.Title, ticket.Type, ticket.RiskLevel, ticket.Status,
		ticket.Requester, approvers, ticket.ExecutorID, affectedAssets,
		ticket.RollbackPlan, ticket.ScheduledStart, ticket.ScheduledEnd,
		ticket.ActualStart, ticket.ActualEnd, ticket.Description,
		ticket.AIRiskSummary, relatedChangeIDs, ticket.MaintenanceID,
		ticket.CancelReason, ticket.CreatedAt, ticket.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert change ticket: %w", err)
	}
	return nil
}

// GetByID 根据变更单 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *ChangeRepo) GetByID(ctx context.Context, id string) (*biz.ChangeTicket, error) {
	ticket := &biz.ChangeTicket{}
	var approvers, affectedAssets, relatedChangeIDs []byte

	err := r.db.QueryRow(ctx,
		`SELECT id, title, type, risk_level, status,
			requester, approvers, executor_id, affected_assets,
			rollback_plan, scheduled_start, scheduled_end,
			actual_start, actual_end, description,
			ai_risk_summary, related_change_ids, maintenance_id,
			cancel_reason, created_at, updated_at
		 FROM change_tickets WHERE id = $1`, id).Scan(
		&ticket.ID, &ticket.Title, &ticket.Type, &ticket.RiskLevel, &ticket.Status,
		&ticket.Requester, &approvers, &ticket.ExecutorID, &affectedAssets,
		&ticket.RollbackPlan, &ticket.ScheduledStart, &ticket.ScheduledEnd,
		&ticket.ActualStart, &ticket.ActualEnd, &ticket.Description,
		&ticket.AIRiskSummary, &relatedChangeIDs, &ticket.MaintenanceID,
		&ticket.CancelReason, &ticket.CreatedAt, &ticket.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get change ticket: %w", err)
	}

	json.Unmarshal(approvers, &ticket.Approvers)
	json.Unmarshal(affectedAssets, &ticket.AffectedAssets)
	json.Unmarshal(relatedChangeIDs, &ticket.RelatedChangeIDs)

	return ticket, nil
}

// Update 更新已有变更单记录。
func (r *ChangeRepo) Update(ctx context.Context, ticket *biz.ChangeTicket) error {
	approvers, _ := json.Marshal(ticket.Approvers)
	affectedAssets, _ := json.Marshal(ticket.AffectedAssets)
	relatedChangeIDs, _ := json.Marshal(ticket.RelatedChangeIDs)

	_, err := r.db.Exec(ctx,
		`UPDATE change_tickets SET
			title = $2, type = $3, risk_level = $4, status = $5,
			requester = $6, approvers = $7, executor_id = $8, affected_assets = $9,
			rollback_plan = $10, scheduled_start = $11, scheduled_end = $12,
			actual_start = $13, actual_end = $14, description = $15,
			ai_risk_summary = $16, related_change_ids = $17, maintenance_id = $18,
			cancel_reason = $19, updated_at = $20
		 WHERE id = $1`,
		ticket.ID, ticket.Title, ticket.Type, ticket.RiskLevel, ticket.Status,
		ticket.Requester, approvers, ticket.ExecutorID, affectedAssets,
		ticket.RollbackPlan, ticket.ScheduledStart, ticket.ScheduledEnd,
		ticket.ActualStart, ticket.ActualEnd, ticket.Description,
		ticket.AIRiskSummary, relatedChangeIDs, ticket.MaintenanceID,
		ticket.CancelReason, ticket.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update change ticket: %w", err)
	}
	return nil
}

// List 根据过滤条件和分页参数查询变更单列表。
func (r *ChangeRepo) List(ctx context.Context, f biz.ListFilter) ([]*biz.ChangeTicket, int64, error) {
	// 动态构建 WHERE 子句
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if f.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*f.Status))
		argIdx++
	}
	if f.Type != nil {
		where += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, string(*f.Type))
		argIdx++
	}
	if f.RiskLevel != nil {
		where += fmt.Sprintf(" AND risk_level = $%d", argIdx)
		args = append(args, string(*f.RiskLevel))
		argIdx++
	}
	if f.Requester != nil {
		where += fmt.Sprintf(" AND requester = $%d", argIdx)
		args = append(args, *f.Requester)
		argIdx++
	}
	if f.StartTime != nil {
		where += fmt.Sprintf(" AND scheduled_start >= $%d", argIdx)
		args = append(args, *f.StartTime)
		argIdx++
	}
	if f.EndTime != nil {
		where += fmt.Sprintf(" AND scheduled_start <= $%d", argIdx)
		args = append(args, *f.EndTime)
		argIdx++
	}

	// 查询总数
	var total int64
	countQuery := "SELECT COUNT(*) FROM change_tickets " + where
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count change tickets: %w", err)
	}

	// 校验排序字段白名单，防止 SQL 注入
	sortCol := "created_at"
	allowedSorts := map[string]bool{
		"created_at": true, "scheduled_start": true, "risk_level": true, "status": true, "updated_at": true,
	}
	if allowedSorts[f.SortBy] {
		sortCol = f.SortBy
	}
	sortOrder := "DESC"
	if f.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	offset := (f.Page - 1) * f.PageSize
	query := fmt.Sprintf(
		`SELECT id, title, type, risk_level, status,
			requester, approvers, executor_id, affected_assets,
			rollback_plan, scheduled_start, scheduled_end,
			actual_start, actual_end, description,
			ai_risk_summary, related_change_ids, maintenance_id,
			cancel_reason, created_at, updated_at
		 FROM change_tickets %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, sortCol, sortOrder, argIdx, argIdx+1,
	)
	args = append(args, f.PageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list change tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*biz.ChangeTicket
	for rows.Next() {
		ticket := &biz.ChangeTicket{}
		var approvers, affectedAssets, relatedChangeIDs []byte
		if err := rows.Scan(
			&ticket.ID, &ticket.Title, &ticket.Type, &ticket.RiskLevel, &ticket.Status,
			&ticket.Requester, &approvers, &ticket.ExecutorID, &affectedAssets,
			&ticket.RollbackPlan, &ticket.ScheduledStart, &ticket.ScheduledEnd,
			&ticket.ActualStart, &ticket.ActualEnd, &ticket.Description,
			&ticket.AIRiskSummary, &relatedChangeIDs, &ticket.MaintenanceID,
			&ticket.CancelReason, &ticket.CreatedAt, &ticket.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan change ticket: %w", err)
		}
		json.Unmarshal(approvers, &ticket.Approvers)
		json.Unmarshal(affectedAssets, &ticket.AffectedAssets)
		json.Unmarshal(relatedChangeIDs, &ticket.RelatedChangeIDs)
		tickets = append(tickets, ticket)
	}

	return tickets, total, nil
}

// FindConflicts 查找与指定时间段和资产列表存在冲突的变更单。
// 冲突条件：时间段重叠 AND 资产有交集，排除终态变更单和自身。
func (r *ChangeRepo) FindConflicts(ctx context.Context, start, end time.Time, assets []string, excludeID string) ([]*biz.ChangeTicket, error) {
	assetsJSON, _ := json.Marshal(assets)

	// 使用 JSONB 操作符检测资产交集：affected_assets ?| array[...]
	// 时间重叠条件：A.start < B.end AND A.end > B.start
	query := `
		SELECT id, title, type, risk_level, status,
			requester, approvers, executor_id, affected_assets,
			rollback_plan, scheduled_start, scheduled_end,
			actual_start, actual_end, description,
			ai_risk_summary, related_change_ids, maintenance_id,
			cancel_reason, created_at, updated_at
		FROM change_tickets
		WHERE status NOT IN ('completed', 'cancelled', 'rejected')
		  AND scheduled_start < $1
		  AND scheduled_end > $2
		  AND id != $3
		  AND affected_assets ?| (SELECT array_agg(value) FROM jsonb_array_elements_text($4::jsonb))
	`

	rows, err := r.db.Query(ctx, query, end, start, excludeID, assetsJSON)
	if err != nil {
		return nil, fmt.Errorf("find conflicts: %w", err)
	}
	defer rows.Close()

	var tickets []*biz.ChangeTicket
	for rows.Next() {
		ticket := &biz.ChangeTicket{}
		var approvers, affectedAssets, relatedChangeIDs []byte
		if err := rows.Scan(
			&ticket.ID, &ticket.Title, &ticket.Type, &ticket.RiskLevel, &ticket.Status,
			&ticket.Requester, &approvers, &ticket.ExecutorID, &affectedAssets,
			&ticket.RollbackPlan, &ticket.ScheduledStart, &ticket.ScheduledEnd,
			&ticket.ActualStart, &ticket.ActualEnd, &ticket.Description,
			&ticket.AIRiskSummary, &relatedChangeIDs, &ticket.MaintenanceID,
			&ticket.CancelReason, &ticket.CreatedAt, &ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		json.Unmarshal(approvers, &ticket.Approvers)
		json.Unmarshal(affectedAssets, &ticket.AffectedAssets)
		json.Unmarshal(relatedChangeIDs, &ticket.RelatedChangeIDs)
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// ListByTimeRange 查询指定时间范围内的变更单（用于变更日历）。
func (r *ChangeRepo) ListByTimeRange(ctx context.Context, start, end time.Time) ([]*biz.ChangeTicket, error) {
	query := `
		SELECT id, title, type, risk_level, status,
			requester, approvers, executor_id, affected_assets,
			rollback_plan, scheduled_start, scheduled_end,
			actual_start, actual_end, description,
			ai_risk_summary, related_change_ids, maintenance_id,
			cancel_reason, created_at, updated_at
		FROM change_tickets
		WHERE status NOT IN ('cancelled', 'rejected')
		  AND scheduled_start <= $1
		  AND scheduled_end >= $2
		ORDER BY scheduled_start ASC
	`

	rows, err := r.db.Query(ctx, query, end, start)
	if err != nil {
		return nil, fmt.Errorf("list by time range: %w", err)
	}
	defer rows.Close()

	var tickets []*biz.ChangeTicket
	for rows.Next() {
		ticket := &biz.ChangeTicket{}
		var approvers, affectedAssets, relatedChangeIDs []byte
		if err := rows.Scan(
			&ticket.ID, &ticket.Title, &ticket.Type, &ticket.RiskLevel, &ticket.Status,
			&ticket.Requester, &approvers, &ticket.ExecutorID, &affectedAssets,
			&ticket.RollbackPlan, &ticket.ScheduledStart, &ticket.ScheduledEnd,
			&ticket.ActualStart, &ticket.ActualEnd, &ticket.Description,
			&ticket.AIRiskSummary, &relatedChangeIDs, &ticket.MaintenanceID,
			&ticket.CancelReason, &ticket.CreatedAt, &ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan change ticket: %w", err)
		}
		json.Unmarshal(approvers, &ticket.Approvers)
		json.Unmarshal(affectedAssets, &ticket.AffectedAssets)
		json.Unmarshal(relatedChangeIDs, &ticket.RelatedChangeIDs)
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// ApprovalRepo 实现 biz.ApprovalRepo 接口，负责审批记录的数据库持久化。
type ApprovalRepo struct {
	db *pgxpool.Pool
}

// NewApprovalRepo 创建一个新的 ApprovalRepo 实例。
func NewApprovalRepo(db *pgxpool.Pool) *ApprovalRepo {
	return &ApprovalRepo{db: db}
}

// Create 创建一条审批记录。
func (r *ApprovalRepo) Create(ctx context.Context, record *biz.ApprovalRecord) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO approval_records (change_id, approver_id, decision, comment, decided_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		record.ChangeID, record.ApproverID, record.Decision, record.Comment, record.DecidedAt,
	)
	if err != nil {
		return fmt.Errorf("insert approval record: %w", err)
	}
	return nil
}

// ListByChange 查询指定变更单的所有审批记录，按决策时间升序返回。
func (r *ApprovalRepo) ListByChange(ctx context.Context, changeID string) ([]*biz.ApprovalRecord, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, change_id, approver_id, decision, comment, decided_at
		 FROM approval_records WHERE change_id = $1 ORDER BY decided_at ASC`, changeID)
	if err != nil {
		return nil, fmt.Errorf("list approval records: %w", err)
	}
	defer rows.Close()

	var records []*biz.ApprovalRecord
	for rows.Next() {
		record := &biz.ApprovalRecord{}
		if err := rows.Scan(
			&record.ID, &record.ChangeID, &record.ApproverID,
			&record.Decision, &record.Comment, &record.DecidedAt,
		); err != nil {
			return nil, fmt.Errorf("scan approval record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}
