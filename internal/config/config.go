package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv string `env:"APP_ENV" envDefault:"development"`
	Port   int    `env:"PORT" envDefault:"8080"`

	DBHost     string `env:"DB_HOST" envDefault:"localhost"`
	DBPort     int    `env:"DB_PORT" envDefault:"5432"`
	DBUser     string `env:"DB_USER" envDefault:"postgres"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME" envDefault:"smartcourse"`
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
