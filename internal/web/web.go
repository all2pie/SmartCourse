// Package web holds shared Gin helpers so domain handlers stay free of repeated error/binding code.
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/errs"
)

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
