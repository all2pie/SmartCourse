package db

import (
	"time"

	"github.com/google/uuid"
)

// Model is embedded in every persisted entity for the shared id/timestamp columns.
type Model struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
