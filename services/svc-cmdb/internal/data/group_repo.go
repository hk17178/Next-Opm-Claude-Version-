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

// GroupRepo 实现 biz.GroupRepo 接口，负责资产分组的数据库持久化操作。
type GroupRepo struct {
	db *pgxpool.Pool
}

// NewGroupRepo 创建一个新的 GroupRepo 实例。
func NewGroupRepo(db *pgxpool.Pool) *GroupRepo {
	return &GroupRepo{db: db}
}

// Create 插入一条新的资产分组记录，回填 group_id。
func (r *GroupRepo) Create(ctx context.Context, g *biz.AssetGroup) error {
	condJSON, _ := json.Marshal(g.Conditions)
	membersJSON, _ := json.Marshal(g.StaticMembers)

	err := r.db.QueryRow(ctx,
		`INSERT INTO asset_groups (name, group_type, conditions, static_members, member_count, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING group_id`,
		g.Name, g.GroupType, condJSON, membersJSON, g.MemberCount, time.Now(), time.Now(),
	).Scan(&g.GroupID)
	if err != nil {
		return fmt.Errorf("insert group: %w", err)
	}
	return nil
}

// GetByID 根据分组 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *GroupRepo) GetByID(ctx context.Context, id string) (*biz.AssetGroup, error) {
	g := &biz.AssetGroup{}
	var condJSON, membersJSON []byte

	err := r.db.QueryRow(ctx,
		`SELECT group_id, name, group_type, conditions, static_members, member_count, created_at, updated_at
		 FROM asset_groups WHERE group_id = $1`, id).Scan(
		&g.GroupID, &g.Name, &g.GroupType, &condJSON, &membersJSON,
		&g.MemberCount, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get group: %w", err)
	}
	json.Unmarshal(condJSON, &g.Conditions)
	json.Unmarshal(membersJSON, &g.StaticMembers)
	return g, nil
}

// List 返回所有资产分组列表，按名称升序排列。
func (r *GroupRepo) List(ctx context.Context) ([]*biz.AssetGroup, error) {
	rows, err := r.db.Query(ctx,
		`SELECT group_id, name, group_type, conditions, static_members, member_count, created_at, updated_at
		 FROM asset_groups ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []*biz.AssetGroup
	for rows.Next() {
		g := &biz.AssetGroup{}
		var condJSON, membersJSON []byte
		if err := rows.Scan(
			&g.GroupID, &g.Name, &g.GroupType, &condJSON, &membersJSON,
			&g.MemberCount, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		json.Unmarshal(condJSON, &g.Conditions)
		json.Unmarshal(membersJSON, &g.StaticMembers)
		groups = append(groups, g)
	}
	return groups, nil
}

// Update 更新分组信息并刷新 updated_at 时间戳。
func (r *GroupRepo) Update(ctx context.Context, g *biz.AssetGroup) error {
	condJSON, _ := json.Marshal(g.Conditions)
	membersJSON, _ := json.Marshal(g.StaticMembers)
	g.UpdatedAt = time.Now()

	_, err := r.db.Exec(ctx,
		`UPDATE asset_groups SET
			name=$2, group_type=$3, conditions=$4, static_members=$5,
			member_count=$6, updated_at=$7
		 WHERE group_id = $1`,
		g.GroupID, g.Name, g.GroupType, condJSON, membersJSON,
		g.MemberCount, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	return nil
}

// Delete 根据分组 ID 删除一条记录。
func (r *GroupRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM asset_groups WHERE group_id = $1", id)
	return err
}

// allowedExprFields 定义条件表达式中允许使用的字段名到数据库列名的映射。
var allowedExprFields = map[string]string{
	"asset_type":  "asset_type",
	"status":      "status",
	"grade":       "grade",
	"environment": "environment",
	"region":      "region",
	"hostname":    "hostname",
	"ip":          "ip",
	"datacenter":  "datacenter",
	"os_type":     "asset_subtype",
}

// EvalDynamicMembers 评估动态分组的条件表达式，返回匹配的资产 ID 列表。
// 静态分组直接返回手动指定的成员列表。
// 支持两种条件格式：
//   1. 简单 JSON 键值对: {"asset_type": "server", "status": "active"}
//   2. 表达式字符串: {"expression": "asset_type == \"server\" AND status == \"active\""}
func (r *GroupRepo) EvalDynamicMembers(ctx context.Context, g *biz.AssetGroup) ([]string, error) {
	if g.GroupType == biz.GroupTypeStatic {
		return g.StaticMembers, nil
	}

	condMap, ok := g.Conditions.(map[string]any)
	if !ok {
		return []string{}, nil
	}

	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	// 检查是否使用表达式格式
	if exprStr, ok := condMap["expression"].(string); ok && exprStr != "" {
		parsed, err := biz.ParseExpression(exprStr)
		if err != nil {
			return nil, fmt.Errorf("parse expression: %w", err)
		}
		exprSQL, exprArgs := biz.BuildSQLFromExpression(parsed, allowedExprFields, argIdx)
		if exprSQL != "" {
			where += " AND " + exprSQL
			args = append(args, exprArgs...)
		}
	} else {
		// 简单键值对条件匹配（向后兼容）
		if v, ok := condMap["asset_type"].(string); ok {
			where += fmt.Sprintf(" AND asset_type = $%d", argIdx)
			args = append(args, v)
			argIdx++
		}
		if v, ok := condMap["environment"].(string); ok {
			where += fmt.Sprintf(" AND environment = $%d", argIdx)
			args = append(args, v)
			argIdx++
		}
		if v, ok := condMap["grade"].(string); ok {
			where += fmt.Sprintf(" AND grade = $%d", argIdx)
			args = append(args, v)
			argIdx++
		}
		if v, ok := condMap["status"].(string); ok {
			where += fmt.Sprintf(" AND status = $%d", argIdx)
			args = append(args, v)
			argIdx++
		}
	}

	rows, err := r.db.Query(ctx,
		fmt.Sprintf("SELECT asset_id FROM assets %s", where), args...)
	if err != nil {
		return nil, fmt.Errorf("eval dynamic members: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}
