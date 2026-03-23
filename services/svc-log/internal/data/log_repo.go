// Package data 实现日志服务的数据访问层，提供 PostgreSQL 和 Elasticsearch 的具体存储操作。
package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-log/internal/biz"
)

// LogRepo 使用 PostgreSQL 实现 biz.LogRepository 接口，负责日志配置数据的持久化。
type LogRepo struct {
	pool *pgxpool.Pool
}

// NewLogRepo 创建一个新的 LogRepo 实例。
func NewLogRepo(pool *pgxpool.Pool) *LogRepo {
	return &LogRepo{pool: pool}
}

// --- LogSource CRUD（日志采集源增删改查） ---

func (r *LogRepo) CreateLogSource(ctx context.Context, src *biz.LogSource) error {
	configJSON, err := json.Marshal(src.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO log_sources (source_id, name, source_type, collection_method, config, parse_rule_id, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, src.SourceID, src.Name, src.SourceType, src.CollectionMethod,
		configJSON, src.ParseRuleID, src.Enabled, src.CreatedAt, src.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert log source: %w", err)
	}
	return nil
}

func (r *LogRepo) GetLogSource(ctx context.Context, id string) (*biz.LogSource, error) {
	var src biz.LogSource
	var configJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT source_id, name, source_type, collection_method, config, parse_rule_id, enabled, created_at, updated_at
		FROM log_sources WHERE source_id = $1
	`, id).Scan(&src.SourceID, &src.Name, &src.SourceType, &src.CollectionMethod,
		&configJSON, &src.ParseRuleID, &src.Enabled, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get log source: %w", err)
	}
	json.Unmarshal(configJSON, &src.Config)
	return &src, nil
}

func (r *LogRepo) ListLogSources(ctx context.Context) ([]*biz.LogSource, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source_id, name, source_type, collection_method, config, parse_rule_id, enabled, created_at, updated_at
		FROM log_sources ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list log sources: %w", err)
	}
	defer rows.Close()

	var sources []*biz.LogSource
	for rows.Next() {
		var src biz.LogSource
		var configJSON []byte
		if err := rows.Scan(&src.SourceID, &src.Name, &src.SourceType, &src.CollectionMethod,
			&configJSON, &src.ParseRuleID, &src.Enabled, &src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan log source: %w", err)
		}
		json.Unmarshal(configJSON, &src.Config)
		sources = append(sources, &src)
	}
	return sources, nil
}

func (r *LogRepo) UpdateLogSource(ctx context.Context, src *biz.LogSource) error {
	configJSON, _ := json.Marshal(src.Config)
	_, err := r.pool.Exec(ctx, `
		UPDATE log_sources SET name=$2, source_type=$3, collection_method=$4, config=$5,
		    parse_rule_id=$6, enabled=$7, updated_at=$8
		WHERE source_id=$1
	`, src.SourceID, src.Name, src.SourceType, src.CollectionMethod,
		configJSON, src.ParseRuleID, src.Enabled, src.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update log source: %w", err)
	}
	return nil
}

func (r *LogRepo) DeleteLogSource(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM log_sources WHERE source_id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete log source: %w", err)
	}
	return nil
}

// --- ParseRule CRUD（解析规则增删改查） ---

func (r *LogRepo) CreateParseRule(ctx context.Context, rule *biz.ParseRule) error {
	multilineJSON, _ := json.Marshal(rule.MultilineRule)
	fieldMappingJSON, _ := json.Marshal(rule.FieldMapping)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO parse_rules (rule_id, name, format_type, pattern, multiline_rule, field_mapping, sample_log, enabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rule.RuleID, rule.Name, rule.FormatType, rule.Pattern,
		multilineJSON, fieldMappingJSON, rule.SampleLog, rule.Enabled, rule.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert parse rule: %w", err)
	}
	return nil
}

func (r *LogRepo) GetParseRule(ctx context.Context, id string) (*biz.ParseRule, error) {
	var rule biz.ParseRule
	var multilineJSON, fieldMappingJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT rule_id, name, format_type, pattern, multiline_rule, field_mapping, sample_log, enabled, created_at
		FROM parse_rules WHERE rule_id=$1
	`, id).Scan(&rule.RuleID, &rule.Name, &rule.FormatType, &rule.Pattern,
		&multilineJSON, &fieldMappingJSON, &rule.SampleLog, &rule.Enabled, &rule.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get parse rule: %w", err)
	}
	json.Unmarshal(multilineJSON, &rule.MultilineRule)
	json.Unmarshal(fieldMappingJSON, &rule.FieldMapping)
	return &rule, nil
}

func (r *LogRepo) ListParseRules(ctx context.Context) ([]*biz.ParseRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rule_id, name, format_type, pattern, multiline_rule, field_mapping, sample_log, enabled, created_at
		FROM parse_rules ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list parse rules: %w", err)
	}
	defer rows.Close()

	var rules []*biz.ParseRule
	for rows.Next() {
		var rule biz.ParseRule
		var multilineJSON, fieldMappingJSON []byte
		if err := rows.Scan(&rule.RuleID, &rule.Name, &rule.FormatType, &rule.Pattern,
			&multilineJSON, &fieldMappingJSON, &rule.SampleLog, &rule.Enabled, &rule.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan parse rule: %w", err)
		}
		json.Unmarshal(multilineJSON, &rule.MultilineRule)
		json.Unmarshal(fieldMappingJSON, &rule.FieldMapping)
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *LogRepo) UpdateParseRule(ctx context.Context, rule *biz.ParseRule) error {
	multilineJSON, _ := json.Marshal(rule.MultilineRule)
	fieldMappingJSON, _ := json.Marshal(rule.FieldMapping)
	_, err := r.pool.Exec(ctx, `
		UPDATE parse_rules SET name=$2, format_type=$3, pattern=$4,
		    multiline_rule=$5, field_mapping=$6, sample_log=$7, enabled=$8
		WHERE rule_id=$1
	`, rule.RuleID, rule.Name, rule.FormatType, rule.Pattern,
		multilineJSON, fieldMappingJSON, rule.SampleLog, rule.Enabled)
	if err != nil {
		return fmt.Errorf("update parse rule: %w", err)
	}
	return nil
}

func (r *LogRepo) DeleteParseRule(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM parse_rules WHERE rule_id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete parse rule: %w", err)
	}
	return nil
}

// --- MaskingRule CRUD（脱敏规则增删改查） ---

func (r *LogRepo) CreateMaskingRule(ctx context.Context, rule *biz.MaskingRule) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO masking_rules (rule_id, name, pattern, replacement, priority, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, rule.RuleID, rule.Name, rule.Pattern, rule.Replacement, rule.Priority, rule.Enabled)
	if err != nil {
		return fmt.Errorf("insert masking rule: %w", err)
	}
	return nil
}

func (r *LogRepo) GetMaskingRule(ctx context.Context, id string) (*biz.MaskingRule, error) {
	var rule biz.MaskingRule
	err := r.pool.QueryRow(ctx, `
		SELECT rule_id, name, pattern, replacement, priority, enabled
		FROM masking_rules WHERE rule_id=$1
	`, id).Scan(&rule.RuleID, &rule.Name, &rule.Pattern, &rule.Replacement, &rule.Priority, &rule.Enabled)
	if err != nil {
		return nil, fmt.Errorf("get masking rule: %w", err)
	}
	return &rule, nil
}

func (r *LogRepo) ListMaskingRules(ctx context.Context) ([]*biz.MaskingRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rule_id, name, pattern, replacement, priority, enabled
		FROM masking_rules ORDER BY priority ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list masking rules: %w", err)
	}
	defer rows.Close()

	var rules []*biz.MaskingRule
	for rows.Next() {
		var rule biz.MaskingRule
		if err := rows.Scan(&rule.RuleID, &rule.Name, &rule.Pattern, &rule.Replacement, &rule.Priority, &rule.Enabled); err != nil {
			return nil, fmt.Errorf("scan masking rule: %w", err)
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *LogRepo) UpdateMaskingRule(ctx context.Context, rule *biz.MaskingRule) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE masking_rules SET name=$2, pattern=$3, replacement=$4, priority=$5, enabled=$6
		WHERE rule_id=$1
	`, rule.RuleID, rule.Name, rule.Pattern, rule.Replacement, rule.Priority, rule.Enabled)
	if err != nil {
		return fmt.Errorf("update masking rule: %w", err)
	}
	return nil
}

func (r *LogRepo) DeleteMaskingRule(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM masking_rules WHERE rule_id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete masking rule: %w", err)
	}
	return nil
}

// --- RetentionPolicy CRUD（保留策略增删改查） ---

func (r *LogRepo) CreateRetentionPolicy(ctx context.Context, p *biz.RetentionPolicy) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO retention_policies (policy_id, name, source_type, log_level, hot_days, warm_days, cold_days, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, p.PolicyID, p.Name, p.SourceType, p.LogLevel, p.HotDays, p.WarmDays, p.ColdDays, p.Enabled)
	if err != nil {
		return fmt.Errorf("insert retention policy: %w", err)
	}
	return nil
}

func (r *LogRepo) GetRetentionPolicy(ctx context.Context, id string) (*biz.RetentionPolicy, error) {
	var p biz.RetentionPolicy
	err := r.pool.QueryRow(ctx, `
		SELECT policy_id, name, source_type, log_level, hot_days, warm_days, cold_days, enabled
		FROM retention_policies WHERE policy_id=$1
	`, id).Scan(&p.PolicyID, &p.Name, &p.SourceType, &p.LogLevel, &p.HotDays, &p.WarmDays, &p.ColdDays, &p.Enabled)
	if err != nil {
		return nil, fmt.Errorf("get retention policy: %w", err)
	}
	return &p, nil
}

func (r *LogRepo) ListRetentionPolicies(ctx context.Context) ([]*biz.RetentionPolicy, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT policy_id, name, source_type, log_level, hot_days, warm_days, cold_days, enabled
		FROM retention_policies
	`)
	if err != nil {
		return nil, fmt.Errorf("list retention policies: %w", err)
	}
	defer rows.Close()

	var policies []*biz.RetentionPolicy
	for rows.Next() {
		var p biz.RetentionPolicy
		if err := rows.Scan(&p.PolicyID, &p.Name, &p.SourceType, &p.LogLevel, &p.HotDays, &p.WarmDays, &p.ColdDays, &p.Enabled); err != nil {
			return nil, fmt.Errorf("scan retention policy: %w", err)
		}
		policies = append(policies, &p)
	}
	return policies, nil
}

func (r *LogRepo) UpdateRetentionPolicy(ctx context.Context, p *biz.RetentionPolicy) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE retention_policies SET name=$2, source_type=$3, log_level=$4,
		    hot_days=$5, warm_days=$6, cold_days=$7, enabled=$8
		WHERE policy_id=$1
	`, p.PolicyID, p.Name, p.SourceType, p.LogLevel, p.HotDays, p.WarmDays, p.ColdDays, p.Enabled)
	if err != nil {
		return fmt.Errorf("update retention policy: %w", err)
	}
	return nil
}

func (r *LogRepo) DeleteRetentionPolicy(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM retention_policies WHERE policy_id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete retention policy: %w", err)
	}
	return nil
}

// --- Stream CRUD（日志流增删改查） ---

func (r *LogRepo) CreateStream(ctx context.Context, s *biz.Stream) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO streams (id, name, filter, retention_days, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, s.ID, s.Name, s.Filter, s.RetentionDays, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert stream: %w", err)
	}
	return nil
}

func (r *LogRepo) GetStream(ctx context.Context, id string) (*biz.Stream, error) {
	var s biz.Stream
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, filter, retention_days, created_at FROM streams WHERE id=$1
	`, id).Scan(&s.ID, &s.Name, &s.Filter, &s.RetentionDays, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}
	return &s, nil
}

func (r *LogRepo) ListStreams(ctx context.Context, pageSize int, pageToken string) ([]*biz.Stream, string, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := `SELECT id, name, filter, retention_days, created_at FROM streams`
	args := []any{}
	argIdx := 1

	if pageToken != "" {
		query += fmt.Sprintf(` WHERE id > $%d`, argIdx)
		args = append(args, pageToken)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY id ASC LIMIT $%d`, argIdx)
	args = append(args, pageSize+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list streams: %w", err)
	}
	defer rows.Close()

	var streams []*biz.Stream
	for rows.Next() {
		var s biz.Stream
		if err := rows.Scan(&s.ID, &s.Name, &s.Filter, &s.RetentionDays, &s.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("scan stream: %w", err)
		}
		streams = append(streams, &s)
	}

	var nextToken string
	if len(streams) > pageSize {
		nextToken = streams[pageSize-1].ID
		streams = streams[:pageSize]
	}

	return streams, nextToken, nil
}

func (r *LogRepo) DeleteStream(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM streams WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete stream: %w", err)
	}
	return nil
}
