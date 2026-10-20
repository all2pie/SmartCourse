package identity

import "github.com/all2pie/go-smart-course/internal/platform/postgres"

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

type User struct {
	postgres.Model
	Name  string `gorm:"not null"                  json:"name"`
	Email string `gorm:"uniqueIndex;not null"      json:"email"`
	Role  Role   `gorm:"type:varchar(50);not null" json:"role"`
}
