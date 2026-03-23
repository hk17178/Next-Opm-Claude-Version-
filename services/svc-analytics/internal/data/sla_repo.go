// Package data 提供 svc-analytics 服务的数据访问层实现。
package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opsnexus/svc-analytics/internal/biz"
)

// SLARepoImpl 使用 PostgreSQL 实现 biz.SLARepo 接口，管理 SLA 配置的 CRUD 操作。
type SLARepoImpl struct {
	pool *pgxpool.Pool
}

// NewSLARepo 创建 SLA 仓储实例。
func NewSLARepo(pool *pgxpool.Pool) *SLARepoImpl {
	return &SLARepoImpl{pool: pool}
}

// CreateConfig 创建 SLA 配置并通过 RETURNING 回填自动生成的字段。
func (r *SLARepoImpl) CreateConfig(ctx context.Context, cfg *biz.SLAConfig) error {
	query := `
		INSERT INTO sla_configs (name, dimension, dimension_value, target_percentage, window, exclude_planned)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING config_id, created_at, updated_at`
	return r.pool.QueryRow(ctx, query,
		cfg.Name, cfg.Dimension, cfg.DimensionValue,
		cfg.TargetPercentage, cfg.Window, cfg.ExcludePlanned,
	).Scan(&cfg.ConfigID, &cfg.CreatedAt, &cfg.UpdatedAt)
}

// GetConfig 根据 config_id 查询单个 SLA 配置。
func (r *SLARepoImpl) GetConfig(ctx context.Context, id string) (*biz.SLAConfig, error) {
	var cfg biz.SLAConfig
	query := `
		SELECT config_id, name, dimension, dimension_value, target_percentage,
		       window, exclude_planned, created_at, updated_at
		FROM sla_configs WHERE config_id = $1`
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&cfg.ConfigID, &cfg.Name, &cfg.Dimension, &cfg.DimensionValue,
		&cfg.TargetPercentage, &cfg.Window, &cfg.ExcludePlanned,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get sla config: %w", err)
	}
	return &cfg, nil
}

// ListConfigs 分页查询 SLA 配置列表，支持按维度和维度值过滤。
func (r *SLARepoImpl) ListConfigs(ctx context.Context, filter biz.SLAListFilter) ([]*biz.SLAConfig, int, error) {
	countQuery := `SELECT COUNT(*) FROM sla_configs WHERE 1=1`
	dataQuery := `
		SELECT config_id, name, dimension, dimension_value, target_percentage,
		       window, exclude_planned, created_at, updated_at
		FROM sla_configs WHERE 1=1`

	args := make([]any, 0)
	argIdx := 1

	if filter.Dimension != nil {
		clause := fmt.Sprintf(" AND dimension = $%d", argIdx)
		countQuery += clause
		dataQuery += clause
		args = append(args, *filter.Dimension)
		argIdx++
	}
	if filter.DimensionValue != nil {
		clause := fmt.Sprintf(" AND dimension_value = $%d", argIdx)
		countQuery += clause
		dataQuery += clause
		args = append(args, *filter.DimensionValue)
		argIdx++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sla configs: %w", err)
	}

	dataQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sla configs: %w", err)
	}
	defer rows.Close()

	configs := make([]*biz.SLAConfig, 0)
	for rows.Next() {
		var cfg biz.SLAConfig
		if err := rows.Scan(
			&cfg.ConfigID, &cfg.Name, &cfg.Dimension, &cfg.DimensionValue,
			&cfg.TargetPercentage, &cfg.Window, &cfg.ExcludePlanned,
			&cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan sla config: %w", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, total, nil
}

// UpdateConfig 更新 SLA 配置，自动更新 updated_at 时间戳。
func (r *SLARepoImpl) UpdateConfig(ctx context.Context, cfg *biz.SLAConfig) error {
	query := `
		UPDATE sla_configs
		SET name = $2, dimension = $3, dimension_value = $4, target_percentage = $5,
		    window = $6, exclude_planned = $7, updated_at = now()
		WHERE config_id = $1`
	_, err := r.pool.Exec(ctx, query,
		cfg.ConfigID, cfg.Name, cfg.Dimension, cfg.DimensionValue,
		cfg.TargetPercentage, cfg.Window, cfg.ExcludePlanned,
	)
	return err
}

// DeleteConfig 根据 config_id 删除 SLA 配置。
func (r *SLARepoImpl) DeleteConfig(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sla_configs WHERE config_id = $1`, id)
	return err
}

// ListConfigsByDimension 查询指定维度下的所有 SLA 配置，用于批量 SLA 计算。
func (r *SLARepoImpl) ListConfigsByDimension(ctx context.Context, dimension string) ([]*biz.SLAConfig, error) {
	query := `
		SELECT config_id, name, dimension, dimension_value, target_percentage,
		       window, exclude_planned, created_at, updated_at
		FROM sla_configs WHERE dimension = $1 ORDER BY created_at`
	rows, err := r.pool.Query(ctx, query, dimension)
	if err != nil {
		return nil, fmt.Errorf("list configs by dimension: %w", err)
	}
	defer rows.Close()

	configs := make([]*biz.SLAConfig, 0)
	for rows.Next() {
		var cfg biz.SLAConfig
		if err := rows.Scan(
			&cfg.ConfigID, &cfg.Name, &cfg.Dimension, &cfg.DimensionValue,
			&cfg.TargetPercentage, &cfg.Window, &cfg.ExcludePlanned,
			&cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan sla config: %w", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, nil
}
