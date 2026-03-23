package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-ai/internal/biz"
)

type promptRepo struct {
	pool *pgxpool.Pool
}

func NewPromptRepo(pool *pgxpool.Pool) biz.PromptRepo {
	return &promptRepo{pool: pool}
}

func (r *promptRepo) GetActive(ctx context.Context, scene string) (*biz.Prompt, error) {
	p := &biz.Prompt{}
	err := r.pool.QueryRow(ctx,
		`SELECT prompt_id, scene, version, system_prompt, user_prompt, variables, is_active, feedback_score, created_at
		 FROM prompts WHERE scene = $1 AND is_active = true
		 ORDER BY created_at DESC LIMIT 1`, scene,
	).Scan(&p.ID, &p.Scene, &p.Version, &p.SystemPrompt, &p.UserPrompt, &p.Variables, &p.IsActive, &p.FeedbackScore, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get active prompt for %q: %w", scene, err)
	}
	return p, nil
}

func (r *promptRepo) GetByVersion(ctx context.Context, scene, version string) (*biz.Prompt, error) {
	p := &biz.Prompt{}
	err := r.pool.QueryRow(ctx,
		`SELECT prompt_id, scene, version, system_prompt, user_prompt, variables, is_active, feedback_score, created_at
		 FROM prompts WHERE scene = $1 AND version = $2`, scene, version,
	).Scan(&p.ID, &p.Scene, &p.Version, &p.SystemPrompt, &p.UserPrompt, &p.Variables, &p.IsActive, &p.FeedbackScore, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get prompt: %w", err)
	}
	return p, nil
}

func (r *promptRepo) List(ctx context.Context) ([]*biz.Prompt, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT prompt_id, scene, version, system_prompt, user_prompt, variables, is_active, feedback_score, created_at
		 FROM prompts ORDER BY scene, created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	defer rows.Close()

	var prompts []*biz.Prompt
	for rows.Next() {
		p := &biz.Prompt{}
		err := rows.Scan(&p.ID, &p.Scene, &p.Version, &p.SystemPrompt, &p.UserPrompt, &p.Variables, &p.IsActive, &p.FeedbackScore, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, p)
	}
	return prompts, nil
}

func (r *promptRepo) Create(ctx context.Context, p *biz.Prompt) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO prompts (prompt_id, scene, version, system_prompt, user_prompt, variables, is_active, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.ID, p.Scene, p.Version, p.SystemPrompt, p.UserPrompt, p.Variables, p.IsActive, p.CreatedAt,
	)
	return err
}
