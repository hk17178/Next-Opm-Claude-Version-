package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// DiscoveryRepo 实现 biz.DiscoveryRepo 接口，负责自动发现记录的数据库持久化操作。
type DiscoveryRepo struct {
	db *pgxpool.Pool
}

// NewDiscoveryRepo 创建一个新的 DiscoveryRepo 实例。
func NewDiscoveryRepo(db *pgxpool.Pool) *DiscoveryRepo {
	return &DiscoveryRepo{db: db}
}

// Create 插入一条新的发现记录，回填 record_id 和 discovered_at。
func (r *DiscoveryRepo) Create(ctx context.Context, d *biz.DiscoveryRecord) error {
	rawJSON, _ := json.Marshal(d.RawData)
	err := r.db.QueryRow(ctx,
		`INSERT INTO discovery_records (discovery_method, ip, hostname, detected_type, detected_grade, status, matched_asset_id, raw_data)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING record_id, discovered_at`,
		d.DiscoveryMethod, d.IP, d.Hostname, d.DetectedType, d.DetectedGrade,
		d.Status, d.MatchedAssetID, rawJSON,
	).Scan(&d.RecordID, &d.DiscoveredAt)
	if err != nil {
		return fmt.Errorf("insert discovery record: %w", err)
	}
	return nil
}

// GetByID 根据记录 ID 查询单条发现记录，不存在时返回 (nil, nil)。
func (r *DiscoveryRepo) GetByID(ctx context.Context, id string) (*biz.DiscoveryRecord, error) {
	d := &biz.DiscoveryRecord{}
	var rawJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT record_id, discovery_method, ip, hostname, detected_type, detected_grade,
			status, matched_asset_id, raw_data, discovered_at
		 FROM discovery_records WHERE record_id = $1`, id).Scan(
		&d.RecordID, &d.DiscoveryMethod, &d.IP, &d.Hostname, &d.DetectedType,
		&d.DetectedGrade, &d.Status, &d.MatchedAssetID, &rawJSON, &d.DiscoveredAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get discovery record: %w", err)
	}
	json.Unmarshal(rawJSON, &d.RawData)
	return d, nil
}

// List 根据状态过滤返回分页的发现记录列表，按发现时间倒序排列。
func (r *DiscoveryRepo) List(ctx context.Context, status string, page, pageSize int) ([]*biz.DiscoveryRecord, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM discovery_records "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count discovery records: %w", err)
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(
		`SELECT record_id, discovery_method, ip, hostname, detected_type, detected_grade,
			status, matched_asset_id, raw_data, discovered_at
		 FROM discovery_records %s ORDER BY discovered_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list discovery records: %w", err)
	}
	defer rows.Close()

	var records []*biz.DiscoveryRecord
	for rows.Next() {
		d := &biz.DiscoveryRecord{}
		var rawJSON []byte
		if err := rows.Scan(
			&d.RecordID, &d.DiscoveryMethod, &d.IP, &d.Hostname, &d.DetectedType,
			&d.DetectedGrade, &d.Status, &d.MatchedAssetID, &rawJSON, &d.DiscoveredAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan discovery record: %w", err)
		}
		json.Unmarshal(rawJSON, &d.RawData)
		records = append(records, d)
	}
	return records, total, nil
}

// UpdateStatus 更新发现记录的审核状态，同时可选绑定匹配的资产 ID。
func (r *DiscoveryRepo) UpdateStatus(ctx context.Context, id, status string, matchedAssetID *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE discovery_records SET status = $2, matched_asset_id = $3 WHERE record_id = $1`,
		id, status, matchedAssetID,
	)
	if err != nil {
		return fmt.Errorf("update discovery status: %w", err)
	}
	return nil
}
