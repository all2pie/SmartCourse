// Package auth is the identity contract between the gateway and the services: roles and the forwarded caller.
package auth

import "github.com/google/uuid"

type Role string

const (
	RoleStudent    Role = "student"
	RoleInstructor Role = "instructor"
	RoleAdmin      Role = "admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleStudent, RoleInstructor, RoleAdmin:
		return true
	default:
		return false
	}
}

// Set by the gateway after verifying the JWT; client-sent copies are stripped there.
const (
	HeaderUserID   = "X-User-ID"
	HeaderUserRole = "X-User-Role"
)

// Actor is the authenticated caller of a request.
type Actor struct {
	ID   uuid.UUID
	Role Role
}
