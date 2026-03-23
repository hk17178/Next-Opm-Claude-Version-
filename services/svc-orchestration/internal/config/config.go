// Package config 提供 svc-orchestration 服务的配置管理。
package config

import "os"

// Config 包含编排服务运行所需的全部配置项。
type Config struct {
	HTTPAddr    string // HTTP 监听地址
	PostgresDSN string // PostgreSQL 连接字符串
}

// Load 从环境变量加载配置，若环境变量未设置则使用默认值。
func Load() *Config {
	return &Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8087"),
		PostgresDSN: envOrDefault("POSTGRES_DSN", "postgres://opm:opm@localhost:5432/opm_orchestration?sslmode=disable"),
	}
}

// envOrDefault 读取环境变量，若为空则返回 fallback 默认值。
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
