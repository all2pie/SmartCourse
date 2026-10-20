package gateway

import "net/url"

type Config struct {
	AppEnv      string  `env:"APP_ENV"                       envDefault:"development"`
	Port        int     `env:"GATEWAY_PORT"                  envDefault:"8000"`
	IdentityURL url.URL `env:"GATEWAY_IDENTITY_URL,required"`
	CatalogURL  url.URL `env:"GATEWAY_CATALOG_URL,required"`
	JWTSecret   string  `env:"JWT_SECRET,required"`
}
