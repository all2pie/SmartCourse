// Package postgres is the shared GORM setup for services that own a Postgres database.
package postgres

import (
	"fmt"
	"log"
	"os"
	"time"

	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

	// Postgres stores microseconds; truncating keeps returned structs equal to stored rows.
	db, err := gorm.Open(pgdriver.Open(dsn), &gorm.Config{
		TranslateError: true,
		NowFunc:        func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      logger.Warn,
			// Never log bound values: they include password hashes and token hashes.
			ParameterizedQueries:      true,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return db, nil
}
