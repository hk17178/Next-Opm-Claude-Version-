// Package data provides data access for the incident domain.
package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DBConfig holds database connection parameters.
type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int32
	MaxIdleConns    int32
	ConnMaxLifetime string
}

// DSN builds a PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// NewDB creates a PostgreSQL connection pool.
func NewDB(ctx context.Context, cfg DBConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = cfg.MaxOpenConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	logger.Info("database connected",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName),
	)
	return pool, nil
}
