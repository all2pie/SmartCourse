package identity

import "github.com/all2pie/go-smart-course/internal/platform/auth"

// Access rules from PLAN.md section 2.

func canSeeEmail(a auth.Actor, u *User) bool {
	return a.Role == auth.RoleAdmin || a.ID == u.ID
}

func canSelfRegisterAs(r auth.Role) bool {
	return r == auth.RoleStudent || r == auth.RoleInstructor
}
