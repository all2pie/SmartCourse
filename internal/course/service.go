package course

import (
	"context"

	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/errs"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

var ErrNothingToUpdate = errs.New(errs.Invalid, "no fields to update")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	InstructorID uuid.UUID
	Title        string
	Description  string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Course, error) {
	c := &Course{
		InstructorID: in.InstructorID,
		Title:        in.Title,
		Description:  in.Description,
		Status:       StatusDraft,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Course, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateInput uses pointers so an omitted field is left unchanged, while an empty string clears it.
type UpdateInput struct {
	Title       *string
	Description *string
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Course, error) {
	if in.Title == nil && in.Description == nil {
		return nil, ErrNothingToUpdate
	}

	return s.repo.Update(ctx, id, in)
}

func (s *Service) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Limit <= 0 {
		f.Limit = defaultListLimit
	}
	f.Limit = min(f.Limit, maxListLimit)
	f.Offset = max(f.Offset, 0)

	res, err := s.repo.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	res.Limit, res.Offset = f.Limit, f.Offset

	return res, nil
}
