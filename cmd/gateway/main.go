package main

import (
	"fmt"
	"log"

	"github.com/all2pie/go-smart-course/internal/gateway"
	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/config"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

func main() {
	cfg, err := config.Load[gateway.Config]()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	tokens, err := auth.NewTokens(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("invalid JWT_SECRET: %v", err)
	}

	router, err := web.NewRouter(cfg.AppEnv)
	if err != nil {
		log.Fatalf("failed to create router: %v", err)
	}

	router.Use(gateway.RequestID(), gateway.StripIdentityHeaders())
	gateway.RegisterDocs(router)
	gateway.RegisterRoutes(router, *cfg, tokens)

	log.Printf("gateway starting: env=%s port=%d identity=%s catalog=%s",
		cfg.AppEnv, cfg.Port, cfg.IdentityURL.String(), cfg.CatalogURL.String())
	if err := router.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
