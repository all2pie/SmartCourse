// Package web holds shared Gin helpers so domain handlers stay free of repeated error/binding code.
package web

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

var errUnauthenticated = errs.New(errs.Unauthenticated, "authentication required")

// Actor returns the caller identified by the gateway-forwarded headers.
func Actor(c *gin.Context) (auth.Actor, error) {
	id, err := uuid.Parse(c.GetHeader(auth.HeaderUserID))
	role := auth.Role(c.GetHeader(auth.HeaderUserRole))
	if err != nil || !role.Valid() {
		return auth.Actor{}, errUnauthenticated
	}
	return auth.Actor{ID: id, Role: role}, nil
}

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
func NewRouter(appEnv string) (*gin.Engine, error) {
	if appEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	return router, nil
}

// HandlerFunc is a Gin handler that reports failures by returning an error.
type HandlerFunc func(c *gin.Context) error

// Wrap adapts a HandlerFunc to gin.HandlerFunc, translating returned errors via WriteError.
func Wrap(h HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h(c); err != nil {
			WriteError(c, err)
		}
	}
}

// WriteError maps an errs.Kind to an HTTP status; anything else is a generic 500 so internals never leak.
func WriteError(c *gin.Context, err error) {
	appErr := errs.From(err)
	if appErr == nil || appErr.Kind == errs.Internal {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(statusFor(appErr.Kind), gin.H{"error": appErr.Message})
}

func statusFor(kind errs.Kind) int {
	switch kind {
	case errs.NotFound:
		return http.StatusNotFound
	case errs.Invalid:
		return http.StatusBadRequest
	case errs.Conflict:
		return http.StatusConflict
	case errs.Unauthenticated:
		return http.StatusUnauthorized
	case errs.Forbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// Bind decodes and validates the JSON body into T, returning an errs.Invalid error on failure.
func Bind[T any](c *gin.Context) (T, error) {
	var v T
	if err := c.ShouldBindJSON(&v); err != nil {
		return v, errs.New(errs.Invalid, err.Error())
	}
	return v, nil
}

// BindQuery decodes and validates URL query parameters into T, returning an errs.Invalid error on failure.
func BindQuery[T any](c *gin.Context) (T, error) {
	var v T
	if err := c.ShouldBindQuery(&v); err != nil {
		return v, errs.New(errs.Invalid, err.Error())
	}
	return v, nil
}
