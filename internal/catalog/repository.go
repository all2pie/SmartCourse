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

var ErrNotFound = errs.New(errs.NotFound, "course not found")

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
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Course, error)
	List(ctx context.Context, f ListFilter) (ListResult, error)
}

type mongoRepository struct {
	courses *mongo.Collection
}

func NewRepository(db *mongo.Database) Repository {
	return &mongoRepository{courses: db.Collection("courses")}
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

func (r *mongoRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Course, error) {
	set := bson.D{{Key: "updated_at", Value: now()}}
	if in.Title != nil {
		set = append(set, bson.E{Key: "title", Value: *in.Title})
	}
	if in.Description != nil {
		set = append(set, bson.E{Key: "description", Value: *in.Description})
	}

	var c Course
	err := r.courses.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: set}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&c)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("updating course: %w", err)
	}
	return &c, nil
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
