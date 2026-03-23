package data

import (
	"github.com/opsnexus/svc-notify/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedis(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
