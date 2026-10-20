package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/identity"
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

	router, err := web.NewRouter(cfg.AppEnv)
	if err != nil {
		log.Fatalf("failed to create router: %v", err)
	}

	router.GET("/health/db", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "up"})
	})

	identity.NewHandler(identity.NewService(identity.NewRepository(db))).RegisterRoutes(router)

	log.Printf("identity service starting: env=%s port=%d", cfg.AppEnv, cfg.Port)
	if err := router.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
