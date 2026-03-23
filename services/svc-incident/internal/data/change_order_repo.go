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

// ChangeOrderRepo 实现变更工单的数据库持久化操作。
type ChangeOrderRepo struct {
	db *pgxpool.Pool
}

// NewChangeOrderRepo 创建一个新的 ChangeOrderRepo 实例。
func NewChangeOrderRepo(db *pgxpool.Pool) *ChangeOrderRepo {
	return &ChangeOrderRepo{db: db}
}

// NextID 生成下一个变更工单 ID，格式为 CHG-YYYYMMDD-NNN。
func (r *ChangeOrderRepo) NextID(ctx context.Context) (string, error) {
	today := time.Now().Format("20060102")
	// 复用 incident_sequences 表，使用带前缀的日期键区分不同序列
	dateKey := "CHG-" + today
	var seq int
	err := r.db.QueryRow(ctx,
		`INSERT INTO incident_sequences (date_key, last_seq)
		 VALUES ($1, 1)
		 ON CONFLICT (date_key) DO UPDATE SET last_seq = incident_sequences.last_seq + 1
		 RETURNING last_seq`, dateKey).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate change order id: %w", err)
	}
	return fmt.Sprintf("CHG-%s-%03d", today, seq), nil
}

// Create 插入一条新的变更工单记录。
func (r *ChangeOrderRepo) Create(ctx context.Context, co *biz.ChangeOrder) error {
	planJSON, _ := json.Marshal(co.Plan)
	scheduleJSON, _ := json.Marshal(co.Schedule)
	relatedJSON, _ := json.Marshal(co.RelatedIncidents)

	_, err := r.db.Exec(ctx,
		`INSERT INTO change_orders (
			change_id, title, change_type, risk_level, status,
			requester_id, approver_id, executor_id, plan, schedule,
			result, related_incidents, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		co.ChangeID, co.Title, co.ChangeType, co.RiskLevel, co.Status,
		co.RequesterID, co.ApproverID, co.ExecutorID, planJSON, scheduleJSON,
		nil, relatedJSON, co.CreatedAt, co.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert change order: %w", err)
	}
	return nil
}

// GetByID 根据变更工单 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *ChangeOrderRepo) GetByID(ctx context.Context, id string) (*biz.ChangeOrder, error) {
	co := &biz.ChangeOrder{}
	var planJSON, scheduleJSON, resultJSON, relatedJSON []byte

	err := r.db.QueryRow(ctx,
		`SELECT change_id, title, change_type, risk_level, status,
			requester_id, approver_id, executor_id, plan, schedule,
			result, related_incidents, created_at, updated_at
		 FROM change_orders WHERE change_id = $1`, id).Scan(
		&co.ChangeID, &co.Title, &co.ChangeType, &co.RiskLevel, &co.Status,
		&co.RequesterID, &co.ApproverID, &co.ExecutorID, &planJSON, &scheduleJSON,
		&resultJSON, &relatedJSON, &co.CreatedAt, &co.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get change order: %w", err)
	}

	json.Unmarshal(planJSON, &co.Plan)
	json.Unmarshal(scheduleJSON, &co.Schedule)
	json.Unmarshal(resultJSON, &co.Result)
	json.Unmarshal(relatedJSON, &co.RelatedIncidents)

	return co, nil
}

// Update 更新已有变更工单记录并刷新 updated_at 时间戳。
func (r *ChangeOrderRepo) Update(ctx context.Context, co *biz.ChangeOrder) error {
	planJSON, _ := json.Marshal(co.Plan)
	scheduleJSON, _ := json.Marshal(co.Schedule)
	resultJSON, _ := json.Marshal(co.Result)
	relatedJSON, _ := json.Marshal(co.RelatedIncidents)
	co.UpdatedAt = time.Now()

	_, err := r.db.Exec(ctx,
		`UPDATE change_orders SET
			title=$2, change_type=$3, risk_level=$4, status=$5,
			requester_id=$6, approver_id=$7, executor_id=$8,
			plan=$9, schedule=$10, result=$11,
			related_incidents=$12, updated_at=$13
		 WHERE change_id = $1`,
		co.ChangeID, co.Title, co.ChangeType, co.RiskLevel, co.Status,
		co.RequesterID, co.ApproverID, co.ExecutorID,
		planJSON, scheduleJSON, resultJSON,
		relatedJSON, co.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update change order: %w", err)
	}
	return nil
}

// List 根据状态过滤返回分页的变更工单列表，按创建时间倒序排列。
func (r *ChangeOrderRepo) List(ctx context.Context, status string, page, pageSize int) ([]*biz.ChangeOrder, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM change_orders "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count change orders: %w", err)
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(
		`SELECT change_id, title, change_type, risk_level, status,
			requester_id, approver_id, executor_id, plan, schedule,
			result, related_incidents, created_at, updated_at
		 FROM change_orders %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list change orders: %w", err)
	}
	defer rows.Close()

	var orders []*biz.ChangeOrder
	for rows.Next() {
		co := &biz.ChangeOrder{}
		var planJSON, scheduleJSON, resultJSON, relatedJSON []byte
		if err := rows.Scan(
			&co.ChangeID, &co.Title, &co.ChangeType, &co.RiskLevel, &co.Status,
			&co.RequesterID, &co.ApproverID, &co.ExecutorID, &planJSON, &scheduleJSON,
			&resultJSON, &relatedJSON, &co.CreatedAt, &co.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan change order: %w", err)
		}
		json.Unmarshal(planJSON, &co.Plan)
		json.Unmarshal(scheduleJSON, &co.Schedule)
		json.Unmarshal(resultJSON, &co.Result)
		json.Unmarshal(relatedJSON, &co.RelatedIncidents)
		orders = append(orders, co)
	}

	return orders, total, nil
}
