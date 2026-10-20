// Package web holds the Gin setup every service shares.
package web

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/platform/problem"
)

// HeaderRequestID is set by the gateway on every request and echoed in the response.
const HeaderRequestID = "X-Request-ID"

// Health reports 200 when check succeeds and 503 otherwise.
func Health(check func(ctx context.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := check(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "up"})
	}
}

// NewRouter returns a Gin engine with the settings every service shares, plus GET /ping.
// Unknown routes, wrong methods and panics answer with problem details.
func NewRouter(appEnv string) (*gin.Engine, error) {
	if appEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}

	router.Use(gin.Logger(), gin.CustomRecovery(func(c *gin.Context, _ any) {
		problem.Write(c.Writer, http.StatusInternalServerError, "internal server error")
		c.Abort()
	}))

	router.HandleMethodNotAllowed = true
	router.NoRoute(func(c *gin.Context) {
		problem.Write(c.Writer, http.StatusNotFound, "no such route")
	})
	router.NoMethod(func(c *gin.Context) {
		problem.Write(c.Writer, http.StatusMethodNotAllowed, "method not allowed on this route")
	})

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	return router, nil
}
