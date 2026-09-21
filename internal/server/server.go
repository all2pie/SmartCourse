package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/all2pie/go-smart-course/internal/config"
)

func Run(cfg *config.Config, db *gorm.DB) error {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		return err
	}

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	router.GET("/health/db", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(503, gin.H{"status": "down"})
			return
		}
		c.JSON(200, gin.H{"status": "up"})
	})

	return router.Run(fmt.Sprintf(":%d", cfg.Port))
}
