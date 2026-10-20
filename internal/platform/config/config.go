package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Load reads .env (if present) and parses the environment into the service's own config struct T.
func Load[T any]() (*T, error) {
	_ = godotenv.Load()

	cfg, err := env.ParseAs[T]()
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}
