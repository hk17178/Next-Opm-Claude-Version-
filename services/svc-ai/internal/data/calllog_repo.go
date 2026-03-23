package data

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-ai/internal/biz"
)

// callLogRepo 实现 biz.CallLogRepo 接口，将 AI 调用日志持久化到 PostgreSQL。
type callLogRepo struct {
	pool *pgxpool.Pool
}

// NewCallLogRepo 创建调用日志仓储实例。
func NewCallLogRepo(pool *pgxpool.Pool) biz.CallLogRepo {
	return &callLogRepo{pool: pool}
}

// Create 将一条 AI 调用日志插入 ai_call_logs 表。
func (r *callLogRepo) Create(ctx context.Context, log *biz.AICallLog) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ai_call_logs (call_id, analysis_id, model_id, scene, prompt_version,
		 input_tokens, output_tokens, latency_ms, status, feedback, input_hash, output_summary, error_message, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		log.ID, log.AnalysisID, log.ModelID, log.Scene, log.PromptVersion,
		log.InputTokens, log.OutputTokens, log.LatencyMs, log.Status,
		log.Feedback, log.InputHash, log.OutputSummary, log.ErrorMessage, log.CreatedAt,
	)
	return err
}
