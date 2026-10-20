// Package problem writes errors as RFC 9457 problem details. It reuses Huma's ErrorModel so
// operation errors, Huma's own validation errors and plain HTTP handlers share one shape.
package problem

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

const ContentType = "application/problem+json"

var humaNewError = huma.NewError

// NewError replaces huma.NewError so error details never echo submitted values (passwords, tokens).
func NewError(status int, msg string, errs ...error) huma.StatusError {
	se := humaNewError(status, msg, errs...)
	if m, ok := se.(*huma.ErrorModel); ok {
		for i, d := range m.Errors {
			if d != nil && d.Value != nil {
				c := *d
				c.Value = nil
				m.Errors[i] = &c
			}
		}
	}
	return se
}

// From converts a domain error into a problem. Unknown errors are logged and become a bare 500
// so internals never reach clients.
func From(err error) huma.StatusError {
	if se, ok := errors.AsType[huma.StatusError](err); ok {
		return se
	}

	e := errs.From(err)
	if e == nil || e.Kind == errs.Internal {
		log.Printf("internal error: %v", err)
		return huma.NewError(http.StatusInternalServerError, "internal server error")
	}
	return huma.NewError(StatusFor(e.Kind), e.Message)
}

func StatusFor(kind errs.Kind) int {
	switch kind {
	case errs.NotFound:
		return http.StatusNotFound
	case errs.Invalid:
		return http.StatusUnprocessableEntity
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

// Write sends a problem from plain net/http code: gateway errors, router fallbacks, panic recovery.
func Write(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&huma.ErrorModel{
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
}
