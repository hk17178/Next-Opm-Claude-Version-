// Package data 实现 CMDB 领域的数据持久化层，基于 PostgreSQL 存储资产、关系、分组、维度和发现记录。
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// AssetRepo 实现 biz.AssetRepo 接口，负责资产的数据库持久化操作。
type AssetRepo struct {
	db *pgxpool.Pool
}

// NewAssetRepo 创建一个新的 AssetRepo 实例。
func NewAssetRepo(db *pgxpool.Pool) *AssetRepo {
	return &AssetRepo{db: db}
}

// Create 插入一条新资产记录，将 JSON 类型字段序列化后存入数据库，并回填生成的 asset_id。
func (r *AssetRepo) Create(ctx context.Context, a *biz.Asset) error {
	buJSON, _ := json.Marshal(a.BusinessUnits)
	tagsJSON, _ := json.Marshal(a.Tags)
	customJSON, _ := json.Marshal(a.CustomDimensions)
	var gradeScoreJSON []byte
	if a.GradeScore != nil {
		gradeScoreJSON, _ = json.Marshal(a.GradeScore)
	}

	err := r.db.QueryRow(ctx,
		`INSERT INTO assets (
			hostname, ip, asset_type, asset_subtype, business_units,
			organization, environment, region, datacenter, grade,
			grade_score, status, tags, custom_dimensions, discovered_by,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING asset_id`,
		a.Hostname, a.IP, a.AssetType, a.AssetSubtype, buJSON,
		a.Organization, a.Environment, a.Region, a.Datacenter, a.Grade,
		gradeScoreJSON, a.Status, tagsJSON, customJSON, a.DiscoveredBy,
		time.Now(), time.Now(),
	).Scan(&a.AssetID)
	if err != nil {
		return fmt.Errorf("insert asset: %w", err)
	}
	return nil
}

// GetByID 根据资产 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *AssetRepo) GetByID(ctx context.Context, id string) (*biz.Asset, error) {
	a := &biz.Asset{}
	var buJSON, tagsJSON, customJSON, gradeScoreJSON []byte

	err := r.db.QueryRow(ctx,
		`SELECT asset_id, hostname, ip, asset_type, asset_subtype, business_units,
			organization, environment, region, datacenter, grade,
			grade_score, status, tags, custom_dimensions, discovered_by,
			created_at, updated_at
		 FROM assets WHERE asset_id = $1`, id).Scan(
		&a.AssetID, &a.Hostname, &a.IP, &a.AssetType, &a.AssetSubtype, &buJSON,
		&a.Organization, &a.Environment, &a.Region, &a.Datacenter, &a.Grade,
		&gradeScoreJSON, &a.Status, &tagsJSON, &customJSON, &a.DiscoveredBy,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}

	json.Unmarshal(buJSON, &a.BusinessUnits)
	json.Unmarshal(tagsJSON, &a.Tags)
	json.Unmarshal(customJSON, &a.CustomDimensions)
	if gradeScoreJSON != nil {
		json.Unmarshal(gradeScoreJSON, &a.GradeScore)
	}

	return a, nil
}

// Update 将资产的所有字段更新到数据库，同时刷新 updated_at 时间戳。
func (r *AssetRepo) Update(ctx context.Context, a *biz.Asset) error {
	buJSON, _ := json.Marshal(a.BusinessUnits)
	tagsJSON, _ := json.Marshal(a.Tags)
	customJSON, _ := json.Marshal(a.CustomDimensions)
	var gradeScoreJSON []byte
	if a.GradeScore != nil {
		gradeScoreJSON, _ = json.Marshal(a.GradeScore)
	}
	a.UpdatedAt = time.Now()

	_, err := r.db.Exec(ctx,
		`UPDATE assets SET
			hostname=$2, ip=$3, asset_type=$4, asset_subtype=$5, business_units=$6,
			organization=$7, environment=$8, region=$9, datacenter=$10, grade=$11,
			grade_score=$12, status=$13, tags=$14, custom_dimensions=$15, discovered_by=$16,
			updated_at=$17
		 WHERE asset_id = $1`,
		a.AssetID, a.Hostname, a.IP, a.AssetType, a.AssetSubtype, buJSON,
		a.Organization, a.Environment, a.Region, a.Datacenter, a.Grade,
		gradeScoreJSON, a.Status, tagsJSON, customJSON, a.DiscoveredBy,
		a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update asset: %w", err)
	}
	return nil
}

// Delete 根据资产 ID 删除一条记录。
func (r *AssetRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM assets WHERE asset_id = $1", id)
	return err
}

// List 根据过滤条件动态构建 SQL 查询，返回分页的资产列表和总数。
func (r *AssetRepo) List(ctx context.Context, f biz.AssetListFilter) ([]*biz.Asset, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if f.AssetType != nil {
		where += fmt.Sprintf(" AND asset_type = $%d", argIdx)
		args = append(args, *f.AssetType)
		argIdx++
	}
	if f.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *f.Status)
		argIdx++
	}
	if f.Grade != nil {
		where += fmt.Sprintf(" AND grade = $%d", argIdx)
		args = append(args, *f.Grade)
		argIdx++
	}
	if f.Environment != nil {
		where += fmt.Sprintf(" AND environment = $%d", argIdx)
		args = append(args, *f.Environment)
		argIdx++
	}
	if f.BusinessUnit != nil {
		where += fmt.Sprintf(" AND business_units @> $%d::jsonb", argIdx)
		args = append(args, fmt.Sprintf(`["%s"]`, *f.BusinessUnit))
		argIdx++
	}
	if f.Search != nil && *f.Search != "" {
		where += fmt.Sprintf(" AND (hostname ILIKE $%d OR ip ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+*f.Search+"%")
		argIdx++
	}

	// 先查询满足条件的总数
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM assets "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}

	// 校验排序字段，防止 SQL 注入
	sortCol := "created_at"
	allowedSorts := map[string]bool{
		"created_at": true, "hostname": true, "ip": true,
		"asset_type": true, "grade": true, "status": true, "updated_at": true,
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
		`SELECT asset_id, hostname, ip, asset_type, asset_subtype, business_units,
			organization, environment, region, datacenter, grade,
			grade_score, status, tags, custom_dimensions, discovered_by,
			created_at, updated_at
		 FROM assets %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, sortCol, sortOrder, argIdx, argIdx+1,
	)
	args = append(args, f.PageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []*biz.Asset
	for rows.Next() {
		a := &biz.Asset{}
		var buJSON, tagsJSON, customJSON, gradeScoreJSON []byte
		if err := rows.Scan(
			&a.AssetID, &a.Hostname, &a.IP, &a.AssetType, &a.AssetSubtype, &buJSON,
			&a.Organization, &a.Environment, &a.Region, &a.Datacenter, &a.Grade,
			&gradeScoreJSON, &a.Status, &tagsJSON, &customJSON, &a.DiscoveredBy,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan asset: %w", err)
		}
		json.Unmarshal(buJSON, &a.BusinessUnits)
		json.Unmarshal(tagsJSON, &a.Tags)
		json.Unmarshal(customJSON, &a.CustomDimensions)
		if gradeScoreJSON != nil {
			json.Unmarshal(gradeScoreJSON, &a.GradeScore)
		}
		assets = append(assets, a)
	}

	return assets, total, nil
}

// FindByHostnameOrIP 根据主机名或 IP 查找资产，用于自动发现时的去重匹配。
func (r *AssetRepo) FindByHostnameOrIP(ctx context.Context, hostname, ip string) (*biz.Asset, error) {
	a := &biz.Asset{}
	var buJSON, tagsJSON, customJSON, gradeScoreJSON []byte

	err := r.db.QueryRow(ctx,
		`SELECT asset_id, hostname, ip, asset_type, asset_subtype, business_units,
			organization, environment, region, datacenter, grade,
			grade_score, status, tags, custom_dimensions, discovered_by,
			created_at, updated_at
		 FROM assets
		 WHERE (hostname = $1 AND $1 != '') OR (ip = $2 AND $2 != '')
		 LIMIT 1`, hostname, ip).Scan(
		&a.AssetID, &a.Hostname, &a.IP, &a.AssetType, &a.AssetSubtype, &buJSON,
		&a.Organization, &a.Environment, &a.Region, &a.Datacenter, &a.Grade,
		&gradeScoreJSON, &a.Status, &tagsJSON, &customJSON, &a.DiscoveredBy,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find asset by hostname/ip: %w", err)
	}
	json.Unmarshal(buJSON, &a.BusinessUnits)
	json.Unmarshal(tagsJSON, &a.Tags)
	json.Unmarshal(customJSON, &a.CustomDimensions)
	if gradeScoreJSON != nil {
		json.Unmarshal(gradeScoreJSON, &a.GradeScore)
	}
	return a, nil
}
