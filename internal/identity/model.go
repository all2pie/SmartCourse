package identity

import (
	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/postgres"
)

type User struct {
	postgres.Model
	Name         string    `gorm:"not null"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"                  json:"-"`
	Role         auth.Role `gorm:"type:varchar(50);not null"`
}
