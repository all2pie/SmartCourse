package course

import (
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/db"
)

type Status string

const (
	StatusDraft Status = "draft"
	StatusReady Status = "ready"
)

type Course struct {
	db.Model
	InstructorID uuid.UUID `gorm:"type:uuid;not null"        json:"instructor_id"`
	Title        string    `gorm:"not null"                  json:"title"`
	Description  string    `gorm:"not null;default:''"       json:"description"`
	Status       Status    `gorm:"type:varchar(20);not null" json:"status"`
}
