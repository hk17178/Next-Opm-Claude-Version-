package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-notify/internal/biz"
)

type broadcastRuleRepo struct {
	pool *pgxpool.Pool
}

func NewBroadcastRuleRepo(pool *pgxpool.Pool) biz.BroadcastRuleRepo {
	return &broadcastRuleRepo{pool: pool}
}

func (r *broadcastRuleRepo) Create(ctx context.Context, rule *biz.BroadcastRule) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO broadcast_rules (rule_id, bot_id, event_node, severity_filter, frequency, template, enabled, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rule.ID, rule.BotID, rule.EventNode, rule.SeverityFilter,
		rule.Frequency, rule.Template, rule.Enabled, rule.CreatedAt,
	)
	return err
}

func (r *broadcastRuleRepo) ListByNode(ctx context.Context, node string) ([]*biz.BroadcastRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT rule_id, bot_id, event_node, severity_filter, frequency, template, enabled, created_at
		 FROM broadcast_rules WHERE event_node = $1 AND enabled = true`, node,
	)
	if err != nil {
		return nil, fmt.Errorf("list rules by node: %w", err)
	}
	defer rows.Close()

	var rules []*biz.BroadcastRule
	for rows.Next() {
		rule := &biz.BroadcastRule{}
		err := rows.Scan(
			&rule.ID, &rule.BotID, &rule.EventNode, &rule.SeverityFilter,
			&rule.Frequency, &rule.Template, &rule.Enabled, &rule.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *broadcastRuleRepo) List(ctx context.Context) ([]*biz.BroadcastRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT rule_id, bot_id, event_node, severity_filter, frequency, template, enabled, created_at
		 FROM broadcast_rules ORDER BY event_node, created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*biz.BroadcastRule
	for rows.Next() {
		rule := &biz.BroadcastRule{}
		err := rows.Scan(
			&rule.ID, &rule.BotID, &rule.EventNode, &rule.SeverityFilter,
			&rule.Frequency, &rule.Template, &rule.Enabled, &rule.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
