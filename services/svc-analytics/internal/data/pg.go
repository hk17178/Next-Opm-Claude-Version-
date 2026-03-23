package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PGConfig holds PostgreSQL connection parameters.
type PGConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int32
}

// DSN builds a PostgreSQL connection string.
func (c PGConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// NewPG creates a PostgreSQL connection pool.
func NewPG(ctx context.Context, cfg PGConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = cfg.MaxOpenConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping pg: %w", err)
	}

	logger.Info("postgres connected",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName),
	)
	return pool, nil
}
