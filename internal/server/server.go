package server

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/config"
)

func Run(cfg *config.Config) error {
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

	return router.Run(fmt.Sprintf(":%d", cfg.Port))
}
