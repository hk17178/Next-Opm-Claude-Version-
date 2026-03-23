package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-ai/internal/biz"
)

type modelRepo struct {
	pool *pgxpool.Pool
}

func NewModelRepo(pool *pgxpool.Pool) biz.ModelRepo {
	return &modelRepo{pool: pool}
}

func (r *modelRepo) GetByID(ctx context.Context, id uuid.UUID) (*biz.AIModel, error) {
	m := &biz.AIModel{}
	err := r.pool.QueryRow(ctx,
		`SELECT model_id, name, provider, deployment_type, api_endpoint, api_key_encrypted,
		        local_endpoint, local_model_name, parameters, rate_limit_qps,
		        enabled, health_status, last_health_check, created_at
		 FROM ai_models WHERE model_id = $1`, id,
	).Scan(
		&m.ID, &m.Name, &m.Provider, &m.DeploymentType, &m.APIEndpoint, &m.APIKeyEncrypted,
		&m.LocalEndpoint, &m.LocalModelName, &m.Parameters, &m.RateLimitQPS,
		&m.Enabled, &m.HealthStatus, &m.LastHealthCheck, &m.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get model: %w", err)
	}
	return m, nil
}

func (r *modelRepo) List(ctx context.Context) ([]*biz.AIModel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT model_id, name, provider, deployment_type, api_endpoint,
		        local_endpoint, local_model_name, parameters, rate_limit_qps,
		        enabled, health_status, last_health_check, created_at
		 FROM ai_models ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()

	var models []*biz.AIModel
	for rows.Next() {
		m := &biz.AIModel{}
		err := rows.Scan(
			&m.ID, &m.Name, &m.Provider, &m.DeploymentType, &m.APIEndpoint,
			&m.LocalEndpoint, &m.LocalModelName, &m.Parameters, &m.RateLimitQPS,
			&m.Enabled, &m.HealthStatus, &m.LastHealthCheck, &m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, nil
}

func (r *modelRepo) GetSceneBinding(ctx context.Context, scene string) (*biz.SceneBinding, error) {
	sb := &biz.SceneBinding{}
	err := r.pool.QueryRow(ctx,
		`SELECT binding_id, scene, primary_model_id, fallback_model_id, routing_strategy, prompt_version, created_at
		 FROM scene_bindings WHERE scene = $1`, scene,
	).Scan(&sb.ID, &sb.Scene, &sb.PrimaryModelID, &sb.FallbackModelID, &sb.RoutingStrategy, &sb.PromptVersion, &sb.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get scene binding for %q: %w", scene, err)
	}
	return sb, nil
}

func (r *modelRepo) UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE ai_models SET health_status = $2, last_health_check = now() WHERE model_id = $1`,
		id, status,
	)
	return err
}
