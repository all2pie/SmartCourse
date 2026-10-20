package catalog

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft Status = "draft"
	// Set only by the publish workflow (Week 2).
	StatusPublished Status = "published"
)

type Course struct {
	ID           uuid.UUID `bson:"_id"`
	InstructorID uuid.UUID `bson:"instructor_id"`
	Title        string    `bson:"title"`
	Description  string    `bson:"description"`
	Status       Status    `bson:"status"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}
