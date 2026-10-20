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
	ID           uuid.UUID `bson:"_id"           json:"id"`
	InstructorID uuid.UUID `bson:"instructor_id" json:"instructor_id"`
	Title        string    `bson:"title"         json:"title"`
	Description  string    `bson:"description"   json:"description"`
	Status       Status    `bson:"status"        json:"status"`
	CreatedAt    time.Time `bson:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"    json:"updated_at"`
}
