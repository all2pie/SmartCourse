package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/all2pie/go-smart-course/api"
	"github.com/all2pie/go-smart-course/internal/catalog"
	"github.com/all2pie/go-smart-course/internal/platform/config"
	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
	"github.com/all2pie/go-smart-course/internal/platform/mongodb"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

func main() {
	cfg, err := config.Load[catalog.Config]()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := mongodb.Connect(ctx, cfg.Mongo)
	cancel()
	if err != nil {
		log.Fatalf("failed to connect to mongodb: %v", err)
	}

	router, err := web.NewRouter(cfg.AppEnv)
	if err != nil {
		log.Fatalf("failed to create router: %v", err)
	}

	router.GET("/health/db", web.Health(func(ctx context.Context) error {
		return db.Client().Ping(ctx, nil)
	}))

	catalog.NewHandler(catalog.NewService(catalog.NewRepository(db))).Register(httpapi.New(router, "Catalog", api.Version))

	log.Printf("catalog service starting: env=%s port=%d", cfg.AppEnv, cfg.Port)
	if err := router.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
