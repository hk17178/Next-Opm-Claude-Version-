// Package data provides data access for the Analytics domain.
package data

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
)

// ClickHouseConfig holds ClickHouse connection parameters.
type ClickHouseConfig struct {
	Addrs    []string
	Database string
	Username string
	Password string
}

// NewClickHouse creates a ClickHouse connection.
func NewClickHouse(ctx context.Context, cfg ClickHouseConfig, logger *zap.Logger) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addrs,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	logger.Info("clickhouse connected",
		zap.Strings("addrs", cfg.Addrs),
		zap.String("database", cfg.Database),
	)
	return conn, nil
}
