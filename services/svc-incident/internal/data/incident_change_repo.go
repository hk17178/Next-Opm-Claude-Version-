// Package data - incident_change_repo.go
// 实现事件变更工单关联记录的数据库持久化，对应表 incident_changes。
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// IncidentChangeRepo 实现 biz.IncidentChangeRepo 接口，
// 负责 incident_changes 表的写入和查询操作。
type IncidentChangeRepo struct {
	db *pgxpool.Pool
}

// NewIncidentChangeRepo 创建一个新的 IncidentChangeRepo 实例。
//
// 参数：
//   - db: pgxpool 数据库连接池，由应用启动时注入。
func NewIncidentChangeRepo(db *pgxpool.Pool) *IncidentChangeRepo {
	return &IncidentChangeRepo{db: db}
}

// Add 将一条变更工单关联记录插入 incident_changes 表，并回填生成的 id 和 created_at。
//
// 参数：
//   - ctx: 请求上下文。
//   - change: 变更关联记录，incident_id 和 change_order_id 为必填字段。
//
// 返回值：
//   - error: 数据库写入失败时返回错误，成功时 change.ID 和 change.CreatedAt 被回填。
func (r *IncidentChangeRepo) Add(ctx context.Context, change *biz.IncidentChange) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO incident_changes (incident_id, change_order_id, description, operator_id, operator_name)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		change.IncidentID,
		change.ChangeOrderID,
		nullableStr(change.Description),
		nullableStr(change.OperatorID),
		nullableStr(change.OperatorName),
	).Scan(&change.ID, &change.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert incident change: %w", err)
	}
	return nil
}

// ListByIncident 查询指定事件的所有变更工单关联记录，按关联时间升序排列。
//
// 参数：
//   - ctx: 请求上下文。
//   - incidentID: 目标事件 ID。
//
// 返回值：
//   - []*biz.IncidentChange: 关联记录列表，如无记录则返回空切片（非 nil），避免 JSON 序列化为 null。
//   - error: 数据库查询失败时返回错误。
func (r *IncidentChangeRepo) ListByIncident(ctx context.Context, incidentID string) ([]*biz.IncidentChange, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, incident_id, change_order_id,
		        COALESCE(description, ''), COALESCE(operator_id, ''), COALESCE(operator_name, ''),
		        created_at
		 FROM incident_changes
		 WHERE incident_id = $1
		 ORDER BY created_at ASC`,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list incident changes: %w", err)
	}
	defer rows.Close()

	// 初始化为空切片，确保无数据时 JSON 序列化为 [] 而非 null
	changes := make([]*biz.IncidentChange, 0)
	for rows.Next() {
		c := &biz.IncidentChange{}
		if err := rows.Scan(
			&c.ID, &c.IncidentID, &c.ChangeOrderID,
			&c.Description, &c.OperatorID, &c.OperatorName,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan incident change: %w", err)
		}
		changes = append(changes, c)
	}

	// rows.Err() 会捕获迭代过程中发生的流式错误
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident changes: %w", err)
	}

	return changes, nil
}

// nullableStr 将空字符串转换为 nil，以在 PostgreSQL 侧存储为 NULL 而非空字符串。
// 对 description/operator_id/operator_name 等可选字段使用，保持 NULL 语义一致。
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SetMTTR 将计算好的 MTTR 秒数更新到指定事件记录的 mttr_seconds 列。
// 该方法在事件关闭（StatusClosed）时由 IncidentUsecase.UpdateStatus 调用。
//
// 参数：
//   - ctx: 请求上下文。
//   - incidentID: 事件 ID。
//   - mttrSeconds: MTTR 秒数，由 resolved_at - detected_at 计算得出。
//
// 返回值：
//   - error: 数据库更新失败时返回错误。
//
// 注意：SetMTTR 被定义在此文件中，由 IncidentRepo 实现（通过类型断言或接口扩展）。
// 为避免修改 incident_repo.go，这里提供独立的实现供 IncidentRepo.SetMTTR 委托调用。
func SetMTTROnDB(ctx context.Context, db *pgxpool.Pool, incidentID string, mttrSeconds int64, updatedAt time.Time) error {
	_, err := db.Exec(ctx,
		`UPDATE incidents SET mttr_seconds = $2, updated_at = $3 WHERE incident_id = $1`,
		incidentID, mttrSeconds, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("set mttr_seconds for incident %s: %w", incidentID, err)
	}
	return nil
}
