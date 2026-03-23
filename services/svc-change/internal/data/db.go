// Package data 提供变更管理服务的数据访问层。
package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DBConfig 数据库连接配置。
type DBConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int32
}

// DSN 构建 PostgreSQL 连接字符串。
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// NewDB 创建 PostgreSQL 连接池。
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
