package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

var (
	ErrNotFound = errs.New(errs.NotFound, "course not found")
	// ErrNoRevision means the course has no unpublished edits; it is never returned to clients.
	ErrNoRevision   = errs.New(errs.NotFound, "course has no draft revision")
	ErrEditConflict = errs.New(errs.Conflict, "the course was changed concurrently, retry")
)

type ListFilter struct {
	InstructorID *uuid.UUID
	// PublishedOnly hides unpublished courses, except those owned by OwnerExempt.
	PublishedOnly bool
	OwnerExempt   *uuid.UUID
	Limit         int
	Offset        int
}

type ListResult struct {
	Items  []Course
	Total  int64
	Limit  int
	Offset int
}

type Repository interface {
	Create(ctx context.Context, c *Course) error
	GetByID(ctx context.Context, id uuid.UUID) (*Course, error)
	List(ctx context.Context, f ListFilter) (ListResult, error)

	// GetRevision returns ErrNoRevision if the course has no draft.
	GetRevision(ctx context.Context, courseID uuid.UUID) (*Revision, error)
	// EnsureRevision returns the course's draft, creating it from the course's current content if missing.
	EnsureRevision(ctx context.Context, c *Course) (*Revision, error)
	// SaveRevision stores r if its Version is still current and bumps it; otherwise it returns ErrEditConflict.
	SaveRevision(ctx context.Context, r *Revision) error
}

type mongoRepository struct {
	courses   *mongo.Collection
	revisions *mongo.Collection
}

func NewRepository(db *mongo.Database) Repository {
	return &mongoRepository{courses: db.Collection("courses"), revisions: db.Collection("course_revisions")}
}

// now is truncated to Mongo's millisecond precision so returned values match what is stored.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func (r *mongoRepository) Create(ctx context.Context, c *Course) error {
	c.ID = uuid.New()
	c.CreatedAt = now()
	c.UpdatedAt = c.CreatedAt

	if _, err := r.courses.InsertOne(ctx, c); err != nil {
		return fmt.Errorf("creating course: %w", err)
	}
	return nil
}

func (r *mongoRepository) GetByID(ctx context.Context, id uuid.UUID) (*Course, error) {
	var c Course
	if err := r.courses.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&c); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("fetching course: %w", err)
	}
	return &c, nil
}

func (r *mongoRepository) GetRevision(ctx context.Context, courseID uuid.UUID) (*Revision, error) {
	var rev Revision
	if err := r.revisions.FindOne(ctx, bson.D{{Key: "_id", Value: courseID}}).Decode(&rev); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoRevision
		}
		return nil, fmt.Errorf("fetching revision: %w", err)
	}
	return &rev, nil
}

func (r *mongoRepository) EnsureRevision(ctx context.Context, c *Course) (*Revision, error) {
	ts := now()
	initial := Revision{
		Title: c.Title, Description: c.Description, Modules: c.Modules, CreatedAt: ts, UpdatedAt: ts,
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var rev Revision
	err := r.revisions.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: c.ID}},
		bson.D{{Key: "$setOnInsert", Value: bson.D{
			{Key: "title", Value: initial.Title},
			{Key: "description", Value: initial.Description},
			{Key: "modules", Value: initial.Modules},
			{Key: "version", Value: int64(0)},
			{Key: "created_at", Value: initial.CreatedAt},
			{Key: "updated_at", Value: initial.UpdatedAt},
		}}},
		opts,
	).Decode(&rev)
	// The server retries a lost upsert race itself; a duplicate key that still surfaces is retried by the caller.
	if mongo.IsDuplicateKeyError(err) {
		return nil, ErrEditConflict
	}
	if err != nil {
		return nil, fmt.Errorf("ensuring revision: %w", err)
	}
	return &rev, nil
}

func (r *mongoRepository) SaveRevision(ctx context.Context, rev *Revision) error {
	next := *rev
	next.Version++
	next.UpdatedAt = now()

	res, err := r.revisions.ReplaceOne(ctx,
		bson.D{{Key: "_id", Value: rev.ID}, {Key: "version", Value: rev.Version}}, next)
	if err != nil {
		return fmt.Errorf("saving revision: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrEditConflict
	}

	*rev = next
	return nil
}

func (r *mongoRepository) List(ctx context.Context, f ListFilter) (ListResult, error) {
	filter := bson.D{}
	if f.InstructorID != nil {
		filter = append(filter, bson.E{Key: "instructor_id", Value: *f.InstructorID})
	}
	if f.PublishedOnly {
		if f.OwnerExempt != nil {
			filter = append(filter, bson.E{Key: "$or", Value: bson.A{
				bson.D{{Key: "status", Value: StatusPublished}},
				bson.D{{Key: "instructor_id", Value: *f.OwnerExempt}},
			}})
		} else {
			filter = append(filter, bson.E{Key: "status", Value: StatusPublished})
		}
	}

	total, err := r.courses.CountDocuments(ctx, filter)
	if err != nil {
		return ListResult{}, fmt.Errorf("counting courses: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: 1}}).
		SetSkip(int64(f.Offset)).
		SetLimit(int64(f.Limit))

	cur, err := r.courses.Find(ctx, filter, opts)
	if err != nil {
		return ListResult{}, fmt.Errorf("listing courses: %w", err)
	}

	items := make([]Course, 0, f.Limit)
	if err := cur.All(ctx, &items); err != nil {
		return ListResult{}, fmt.Errorf("decoding courses: %w", err)
	}

	return ListResult{Items: items, Total: total}, nil
}
