package identity

import "github.com/all2pie/go-smart-course/internal/platform/postgres"

type Config struct {
	AppEnv string          `env:"APP_ENV"       envDefault:"development"`
	Port   int             `env:"IDENTITY_PORT" envDefault:"8081"`
	DB     postgres.Config `envPrefix:"IDENTITY_DB_"`
}
