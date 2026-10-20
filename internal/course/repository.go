package course

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/all2pie/go-smart-course/internal/errs"
)

var (
	ErrNotFound           = errs.New(errs.NotFound, "course not found")
	ErrInstructorNotFound = errs.New(errs.Invalid, "instructor does not exist")
)

type Repository interface {
	Create(ctx context.Context, c *Course) error
	GetByID(ctx context.Context, id uuid.UUID) (*Course, error)
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
