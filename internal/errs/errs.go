// Package errs defines transport-agnostic error kinds that domains return and adapters (HTTP, workers) translate.
package errs

import "errors"

type Kind int

const (
	Internal Kind = iota
	NotFound
	Invalid
	Conflict
)

type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string { return e.Message }

// New is meant for package-level sentinels so callers can still use errors.Is.
func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// From returns the first *Error in err's chain, or nil.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}
