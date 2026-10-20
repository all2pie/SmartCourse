package main

import (
	"context"
	"fmt"
	"log"

	"github.com/all2pie/go-smart-course/internal/identity"
	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/config"
	"github.com/all2pie/go-smart-course/internal/platform/postgres"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

func main() {
	cfg, err := config.Load[identity.Config]()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	tokens, err := auth.NewTokens(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("invalid JWT_SECRET: %v", err)
	}

	svc, err := identity.NewService(identity.NewRepository(db), tokens, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	router, err := web.NewRouter(cfg.AppEnv)
	if err != nil {
		log.Fatalf("failed to create router: %v", err)
	}

	router.GET("/health/db", web.Health(func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}))

	identity.NewHandler(svc).RegisterRoutes(router)

	log.Printf("identity service starting: env=%s port=%d", cfg.AppEnv, cfg.Port)
	if err := router.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
