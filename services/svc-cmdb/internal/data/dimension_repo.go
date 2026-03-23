package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// DimensionRepo 实现 biz.DimensionRepo 接口，负责自定义维度的数据库持久化操作。
type DimensionRepo struct {
	db *pgxpool.Pool
}

// NewDimensionRepo 创建一个新的 DimensionRepo 实例。
func NewDimensionRepo(db *pgxpool.Pool) *DimensionRepo {
	return &DimensionRepo{db: db}
}

// Create 插入一条新的自定义维度记录，回填 dim_id 和 created_at。
func (r *DimensionRepo) Create(ctx context.Context, d *biz.CustomDimension) error {
	configJSON, _ := json.Marshal(d.Config)
	err := r.db.QueryRow(ctx,
		`INSERT INTO custom_dimensions (name, display_name, dim_type, config, required, sortable, filterable)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING dim_id, created_at`,
		d.Name, d.DisplayName, d.DimType, configJSON, d.Required, d.Sortable, d.Filterable,
	).Scan(&d.DimID, &d.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert dimension: %w", err)
	}
	return nil
}

// GetByID 根据维度 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *DimensionRepo) GetByID(ctx context.Context, id string) (*biz.CustomDimension, error) {
	d := &biz.CustomDimension{}
	var configJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT dim_id, name, display_name, dim_type, config, required, sortable, filterable, created_at
		 FROM custom_dimensions WHERE dim_id = $1`, id).Scan(
		&d.DimID, &d.Name, &d.DisplayName, &d.DimType, &configJSON,
		&d.Required, &d.Sortable, &d.Filterable, &d.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get dimension: %w", err)
	}
	json.Unmarshal(configJSON, &d.Config)
	return d, nil
}

// GetByName 根据维度唯一名称查询记录，用于创建时的重名校验。
func (r *DimensionRepo) GetByName(ctx context.Context, name string) (*biz.CustomDimension, error) {
	d := &biz.CustomDimension{}
	var configJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT dim_id, name, display_name, dim_type, config, required, sortable, filterable, created_at
		 FROM custom_dimensions WHERE name = $1`, name).Scan(
		&d.DimID, &d.Name, &d.DisplayName, &d.DimType, &configJSON,
		&d.Required, &d.Sortable, &d.Filterable, &d.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get dimension by name: %w", err)
	}
	json.Unmarshal(configJSON, &d.Config)
	return d, nil
}

// List 返回所有自定义维度列表，按名称升序排列。
func (r *DimensionRepo) List(ctx context.Context) ([]*biz.CustomDimension, error) {
	rows, err := r.db.Query(ctx,
		`SELECT dim_id, name, display_name, dim_type, config, required, sortable, filterable, created_at
		 FROM custom_dimensions ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list dimensions: %w", err)
	}
	defer rows.Close()

	var dims []*biz.CustomDimension
	for rows.Next() {
		d := &biz.CustomDimension{}
		var configJSON []byte
		if err := rows.Scan(
			&d.DimID, &d.Name, &d.DisplayName, &d.DimType, &configJSON,
			&d.Required, &d.Sortable, &d.Filterable, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dimension: %w", err)
		}
		json.Unmarshal(configJSON, &d.Config)
		dims = append(dims, d)
	}
	return dims, nil
}

// Update 更新已有维度的配置信息。
func (r *DimensionRepo) Update(ctx context.Context, d *biz.CustomDimension) error {
	configJSON, _ := json.Marshal(d.Config)
	_, err := r.db.Exec(ctx,
		`UPDATE custom_dimensions SET
			display_name=$2, dim_type=$3, config=$4, required=$5, sortable=$6, filterable=$7
		 WHERE dim_id = $1`,
		d.DimID, d.DisplayName, d.DimType, configJSON, d.Required, d.Sortable, d.Filterable,
	)
	if err != nil {
		return fmt.Errorf("update dimension: %w", err)
	}
	return nil
}

// Delete 根据维度 ID 删除一条记录。
func (r *DimensionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM custom_dimensions WHERE dim_id = $1", id)
	return err
}
