package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-ai/internal/biz"
)

// budgetRepo 实现 biz.BudgetRepo 接口，使用 PostgreSQL 管理 AI 模型的 token 预算。
type budgetRepo struct {
	pool *pgxpool.Pool
}

// NewBudgetRepo 创建预算仓储实例。
func NewBudgetRepo(pool *pgxpool.Pool) biz.BudgetRepo {
	return &budgetRepo{pool: pool}
}

// GetOrCreate 获取或创建指定月份和模型的预算记录。
// 使用 INSERT ... ON CONFLICT DO NOTHING 实现幂等创建。
func (r *budgetRepo) GetOrCreate(ctx context.Context, month string, modelID uuid.UUID, defaultLimit int64) (*biz.AIBudget, error) {
	b := &biz.AIBudget{}

	err := r.pool.QueryRow(ctx,
		`INSERT INTO ai_budget (month, model_id, budget_limit)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (month, model_id) DO NOTHING
		 RETURNING month, model_id, tokens_used, budget_limit, alert_sent, exhausted`,
		month, modelID, defaultLimit,
	).Scan(&b.Month, &b.ModelID, &b.TokensUsed, &b.BudgetLimit, &b.AlertSent, &b.Exhausted)

	if err != nil {
		// Row already exists, fetch it
		err = r.pool.QueryRow(ctx,
			`SELECT month, model_id, tokens_used, budget_limit, alert_sent, exhausted
			 FROM ai_budget WHERE month = $1 AND model_id = $2`,
			month, modelID,
		).Scan(&b.Month, &b.ModelID, &b.TokensUsed, &b.BudgetLimit, &b.AlertSent, &b.Exhausted)
		if err != nil {
			return nil, fmt.Errorf("get budget: %w", err)
		}
	}

	return b, nil
}

// IncrementUsage 原子性地增加 token 用量，并自动判断是否触发告警（80%）或耗尽（100%）标记。
func (r *budgetRepo) IncrementUsage(ctx context.Context, month string, modelID uuid.UUID, tokens int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE ai_budget SET tokens_used = tokens_used + $3,
		 alert_sent = CASE WHEN (tokens_used + $3) >= budget_limit * 0.8 THEN true ELSE alert_sent END,
		 exhausted = CASE WHEN (tokens_used + $3) >= budget_limit THEN true ELSE exhausted END
		 WHERE month = $1 AND model_id = $2`,
		month, modelID, tokens,
	)
	return err
}
