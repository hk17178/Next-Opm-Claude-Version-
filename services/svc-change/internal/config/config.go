// Package config 提供变更管理微服务的配置管理功能。
package config

import "os"

// Config 变更管理服务的配置结构。
type Config struct {
	HTTPAddr string // HTTP 服务监听地址
	DBHost   string // 数据库主机
	DBPort   int    // 数据库端口
	DBUser   string // 数据库用户名
	DBPass   string // 数据库密码
	DBName   string // 数据库名称
}

// EnvOrDefault 读取环境变量，若为空则返回默认值。
func EnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
