package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opsnexus/svc-alert/internal/biz"
)

// RuleRepo 实现 biz.RuleRepository 接口，提供告警规则的 PostgreSQL 持久化。
type RuleRepo struct {
	pool *pgxpool.Pool
}

// NewRuleRepo 创建规则仓储实例。
func NewRuleRepo(pool *pgxpool.Pool) *RuleRepo {
	return &RuleRepo{pool: pool}
}

// Create 将告警规则持久化到 alert_rules 表。
func (r *RuleRepo) Create(rule *biz.AlertRule) error {
	notiJSON, _ := json.Marshal(rule.NotificationCh)

	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO alert_rules (rule_id, name, description, layer, rule_type, condition,
		 targets, severity, ironclad, enabled, schedule, cooldown_minutes,
		 notification_channels, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		rule.RuleID, rule.Name, rule.Description, rule.Layer, rule.RuleType,
		rule.Condition, rule.Targets, rule.Severity, rule.Ironclad, rule.Enabled,
		rule.Schedule, rule.CooldownMinutes, notiJSON, rule.CreatedBy,
		rule.CreatedAt, rule.UpdatedAt,
	)
	return err
}

// Update 更新 alert_rules 表中的规则记录。
func (r *RuleRepo) Update(rule *biz.AlertRule) error {
	notiJSON, _ := json.Marshal(rule.NotificationCh)

	_, err := r.pool.Exec(context.Background(),
		`UPDATE alert_rules SET name=$2, description=$3, layer=$4, rule_type=$5,
		 condition=$6, targets=$7, severity=$8, ironclad=$9, enabled=$10,
		 schedule=$11, cooldown_minutes=$12, notification_channels=$13,
		 updated_at=$14 WHERE rule_id=$1`,
		rule.RuleID, rule.Name, rule.Description, rule.Layer, rule.RuleType,
		rule.Condition, rule.Targets, rule.Severity, rule.Ironclad, rule.Enabled,
		rule.Schedule, rule.CooldownMinutes, notiJSON, rule.UpdatedAt,
	)
	return err
}

// Delete 按 ID 删除告警规则。
func (r *RuleRepo) Delete(id string) error {
	_, err := r.pool.Exec(context.Background(), `DELETE FROM alert_rules WHERE rule_id=$1`, id)
	return err
}

// GetByID 按规则 ID 查询单条规则。
func (r *RuleRepo) GetByID(id string) (*biz.AlertRule, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT rule_id, name, description, layer, rule_type, condition, targets,
		 severity, ironclad, enabled, schedule, cooldown_minutes,
		 notification_channels, created_by, created_at, updated_at
		 FROM alert_rules WHERE rule_id=$1`, id)

	return r.scanRule(row)
}

// List 分页查询规则列表，支持按启用状态过滤，使用 keyset 分页。
func (r *RuleRepo) List(enabled *bool, pageSize int, pageToken string) ([]*biz.AlertRule, string, error) {
	query := `SELECT rule_id, name, description, layer, rule_type, condition, targets,
		severity, ironclad, enabled, schedule, cooldown_minutes,
		notification_channels, created_by, created_at, updated_at
		FROM alert_rules`
	args := []interface{}{}
	argIdx := 1

	whereClauses := []string{}
	if enabled != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("enabled = $%d", argIdx))
		args = append(args, *enabled)
		argIdx++
	}
	if pageToken != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("rule_id > $%d", argIdx))
		args = append(args, pageToken)
		argIdx++
	}

	if len(whereClauses) > 0 {
		query += " WHERE "
		for i, clause := range whereClauses {
			if i > 0 {
				query += " AND "
			}
			query += clause
		}
	}

	query += " ORDER BY rule_id"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, pageSize+1) // fetch one extra to determine next_page_token

	rows, err := r.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	rules, err := r.scanRules(rows)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if len(rules) > pageSize {
		nextToken = rules[pageSize].RuleID
		rules = rules[:pageSize]
	}

	return rules, nextToken, nil
}

// ListByLayerAndType 按层级和规则类型查询启用的规则列表。
func (r *RuleRepo) ListByLayerAndType(layer int, ruleType biz.RuleType) ([]*biz.AlertRule, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT rule_id, name, description, layer, rule_type, condition, targets,
		 severity, ironclad, enabled, schedule, cooldown_minutes,
		 notification_channels, created_by, created_at, updated_at
		 FROM alert_rules WHERE layer=$1 AND rule_type=$2 AND enabled=true`, layer, ruleType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules, err := r.scanRules(rows)
	return rules, err
}

// ListEnabled 查询所有启用的规则，按层级和严重等级排序，供告警引擎评估使用。
func (r *RuleRepo) ListEnabled() ([]*biz.AlertRule, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT rule_id, name, description, layer, rule_type, condition, targets,
		 severity, ironclad, enabled, schedule, cooldown_minutes,
		 notification_channels, created_by, created_at, updated_at
		 FROM alert_rules WHERE enabled=true ORDER BY layer, severity`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules, err := r.scanRules(rows)
	return rules, err
}

// scanRule 从单行查询结果扫描为 biz.AlertRule 结构体。
func (r *RuleRepo) scanRule(row pgx.Row) (*biz.AlertRule, error) {
	var rule biz.AlertRule
	var notiJSON []byte

	err := row.Scan(
		&rule.RuleID, &rule.Name, &rule.Description, &rule.Layer, &rule.RuleType,
		&rule.Condition, &rule.Targets, &rule.Severity, &rule.Ironclad, &rule.Enabled,
		&rule.Schedule, &rule.CooldownMinutes, &notiJSON, &rule.CreatedBy,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan rule: %w", err)
	}

	_ = json.Unmarshal(notiJSON, &rule.NotificationCh)
	return &rule, nil
}

// scanRules 从多行查询结果扫描为 biz.AlertRule 切片。
func (r *RuleRepo) scanRules(rows pgx.Rows) ([]*biz.AlertRule, error) {
	var rules []*biz.AlertRule
	for rows.Next() {
		var rule biz.AlertRule
		var notiJSON []byte

		err := rows.Scan(
			&rule.RuleID, &rule.Name, &rule.Description, &rule.Layer, &rule.RuleType,
			&rule.Condition, &rule.Targets, &rule.Severity, &rule.Ironclad, &rule.Enabled,
			&rule.Schedule, &rule.CooldownMinutes, &notiJSON, &rule.CreatedBy,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan rule row: %w", err)
		}

		_ = json.Unmarshal(notiJSON, &rule.NotificationCh)
		rules = append(rules, &rule)
	}
	return rules, rows.Err()
}
