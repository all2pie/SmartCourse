package catalog

import "github.com/all2pie/go-smart-course/internal/platform/mongodb"

type Config struct {
	AppEnv string         `env:"APP_ENV"      envDefault:"development"`
	Port   int            `env:"CATALOG_PORT" envDefault:"8082"`
	Mongo  mongodb.Config `envPrefix:"CATALOG_MONGO_"`
}
