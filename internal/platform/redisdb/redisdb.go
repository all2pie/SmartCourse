// Package redisdb is the shared Redis client setup for services that use Redis.
package redisdb

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Config is embedded by services with an envPrefix, e.g. `envPrefix:"IDENTITY_REDIS_"`.
type Config struct {
	Addr     string `env:"ADDR,required"`
	Password string `env:"PASSWORD"`
	DB       int    `env:"DB"             envDefault:"0"`
}

// Connect returns a client, failing fast if the server is unreachable.
func Connect(ctx context.Context, cfg Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return client, nil
}
