package catalog

import (
	"time"

	"github.com/google/uuid"
)

type LessonType string

const (
	LessonVideo    LessonType = "video"
	LessonDocument LessonType = "document"
	LessonArticle  LessonType = "article"
	LessonLink     LessonType = "external_link"
)

// A lesson holds a URL (video, document, external link) or a body (article), never both.
type Lesson struct {
	ID    uuid.UUID  `bson:"id"`
	Title string     `bson:"title"`
	Type  LessonType `bson:"type"`
	URL   string     `bson:"url,omitempty"`
	Body  string     `bson:"body,omitempty"`
}

type Module struct {
	ID      uuid.UUID `bson:"id"`
	Title   string    `bson:"title"`
	Lessons []Lesson  `bson:"lessons,omitempty"`
}

// Revision is a course's unpublished working copy, created on its first edit (_id is the course ID).
// Version is bumped on every save so concurrent edits are detected instead of overwritten.
type Revision struct {
	ID          uuid.UUID `bson:"_id"`
	Title       string    `bson:"title"`
	Description string    `bson:"description"`
	Modules     []Module  `bson:"modules,omitempty"`
	Version     int64     `bson:"version"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}
