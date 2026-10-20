// Package postgres is the shared GORM setup for services that own a Postgres database.
package postgres

import (
	"fmt"

	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config is embedded by services with an envPrefix, e.g. `envPrefix:"IDENTITY_DB_"`.
type Config struct {
	Host     string `env:"HOST"     envDefault:"localhost"`
	Port     int    `env:"PORT"     envDefault:"5432"`
	User     string `env:"USER"     envDefault:"postgres"`
	Password string `env:"PASSWORD,required"`
	Name     string `env:"NAME,required"`
}

func Connect(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)

	db, err := gorm.Open(pgdriver.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return db, nil
}
