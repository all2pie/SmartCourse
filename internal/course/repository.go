package course

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/all2pie/go-smart-course/internal/errs"
)

var (
	ErrNotFound           = errs.New(errs.NotFound, "course not found")
	ErrInstructorNotFound = errs.New(errs.Invalid, "instructor does not exist")
)

type ListFilter struct {
	InstructorID *uuid.UUID
	Limit        int
	Offset       int
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

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(ctx context.Context, c *Course) error {
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		// instructor_id is the only foreign key on courses.
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return ErrInstructorNotFound
		}
		return fmt.Errorf("creating course: %w", err)
	}
	return nil
}

func (r *gormRepository) GetByID(ctx context.Context, id uuid.UUID) (*Course, error) {
	var c Course
	if err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("fetching course: %w", err)
	}
	return &c, nil
}

func (r *gormRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Course, error) {
	fields := map[string]any{}
	if in.Title != nil {
		fields["title"] = *in.Title
	}
	if in.Description != nil {
		fields["description"] = *in.Description
	}

	var c Course
	// Returning fills c from the updated row, avoiding a second query.
	res := r.db.WithContext(ctx).
		Model(&c).
		Clauses(clause.Returning{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return nil, fmt.Errorf("updating course: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &c, nil
}

func (r *gormRepository) List(ctx context.Context, f ListFilter) (ListResult, error) {
	q := r.db.WithContext(ctx).Model(&Course{})
	if f.InstructorID != nil {
		q = q.Where("instructor_id = ?", *f.InstructorID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("counting courses: %w", err)
	}

	items := make([]Course, 0, f.Limit)
	if err := q.Order("created_at DESC, id").Limit(f.Limit).Offset(f.Offset).Find(&items).Error; err != nil {
		return ListResult{}, fmt.Errorf("listing courses: %w", err)
	}

	return ListResult{Items: items, Total: total}, nil
}
