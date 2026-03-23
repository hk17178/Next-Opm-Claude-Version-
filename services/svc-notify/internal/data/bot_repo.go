// Package data 实现通知服务的数据访问层，提供 PostgreSQL 的具体存储操作。
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-notify/internal/biz"
)

// botRepo 使用 PostgreSQL 实现 biz.BotRepo 接口，负责通知机器人的 CRUD 操作。
type botRepo struct {
	pool *pgxpool.Pool
}

// NewBotRepo 创建一个新的机器人仓储实例。
func NewBotRepo(pool *pgxpool.Pool) biz.BotRepo {
	return &botRepo{pool: pool}
}

func (r *botRepo) Create(ctx context.Context, bot *biz.Bot) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO bots (bot_id, name, channel_type, config, scope, template_type, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		bot.ID, bot.Name, bot.ChannelType, bot.Config, bot.Scope,
		bot.TemplateType, bot.Enabled, bot.CreatedAt, bot.UpdatedAt,
	)
	return err
}

func (r *botRepo) GetByID(ctx context.Context, id uuid.UUID) (*biz.Bot, error) {
	bot := &biz.Bot{}
	err := r.pool.QueryRow(ctx,
		`SELECT bot_id, name, channel_type, config, scope, template_type,
		        health_status, last_health_check, failure_count, enabled, created_at, updated_at
		 FROM bots WHERE bot_id = $1`, id,
	).Scan(
		&bot.ID, &bot.Name, &bot.ChannelType, &bot.Config, &bot.Scope, &bot.TemplateType,
		&bot.HealthStatus, &bot.LastHealthCheck, &bot.FailureCount, &bot.Enabled, &bot.CreatedAt, &bot.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get bot: %w", err)
	}
	return bot, nil
}

func (r *botRepo) List(ctx context.Context) ([]*biz.Bot, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT bot_id, name, channel_type, config, scope, template_type,
		        health_status, last_health_check, failure_count, enabled, created_at, updated_at
		 FROM bots ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []*biz.Bot
	for rows.Next() {
		bot := &biz.Bot{}
		err := rows.Scan(
			&bot.ID, &bot.Name, &bot.ChannelType, &bot.Config, &bot.Scope, &bot.TemplateType,
			&bot.HealthStatus, &bot.LastHealthCheck, &bot.FailureCount, &bot.Enabled, &bot.CreatedAt, &bot.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		bots = append(bots, bot)
	}
	return bots, nil
}

func (r *botRepo) ListEnabled(ctx context.Context) ([]*biz.Bot, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT bot_id, name, channel_type, config, scope, template_type,
		        health_status, last_health_check, failure_count, enabled, created_at, updated_at
		 FROM bots WHERE enabled = true ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []*biz.Bot
	for rows.Next() {
		bot := &biz.Bot{}
		err := rows.Scan(
			&bot.ID, &bot.Name, &bot.ChannelType, &bot.Config, &bot.Scope, &bot.TemplateType,
			&bot.HealthStatus, &bot.LastHealthCheck, &bot.FailureCount, &bot.Enabled, &bot.CreatedAt, &bot.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		bots = append(bots, bot)
	}
	return bots, nil
}

func (r *botRepo) Update(ctx context.Context, bot *biz.Bot) error {
	bot.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE bots SET name = $2, channel_type = $3, config = $4, scope = $5,
		 template_type = $6, enabled = $7, updated_at = $8
		 WHERE bot_id = $1`,
		bot.ID, bot.Name, bot.ChannelType, bot.Config, bot.Scope,
		bot.TemplateType, bot.Enabled, bot.UpdatedAt,
	)
	return err
}

func (r *botRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM bots WHERE bot_id = $1`, id)
	return err
}

func (r *botRepo) UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string, failureCount int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE bots SET health_status = $2, failure_count = $3, last_health_check = now()
		 WHERE bot_id = $1`,
		id, status, failureCount,
	)
	return err
}
