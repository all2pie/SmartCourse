package identity

import (
	"time"

	"github.com/all2pie/go-smart-course/internal/platform/postgres"
)

type Config struct {
	AppEnv          string          `env:"APP_ENV"                    envDefault:"development"`
	Port            int             `env:"IDENTITY_PORT"              envDefault:"8081"`
	DB              postgres.Config `envPrefix:"IDENTITY_DB_"`
	JWTSecret       string          `env:"JWT_SECRET,required"`
	AccessTokenTTL  time.Duration   `env:"IDENTITY_ACCESS_TOKEN_TTL"  envDefault:"15m"`
	RefreshTokenTTL time.Duration   `env:"IDENTITY_REFRESH_TOKEN_TTL" envDefault:"168h"`
}
