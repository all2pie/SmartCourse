package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv string `env:"APP_ENV" envDefault:"development"`
	Port   int    `env:"PORT" envDefault:"8080"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	cfg := &Config{}
	err := env.Parse(cfg)

	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
