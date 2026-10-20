package identity

import (
	"context"

	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

var ErrInvalidRole = errs.New(errs.Invalid, "invalid role")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name  string
	Email string
	Role  Role
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*User, error) {
	if !in.Role.Valid() {
		return nil, ErrInvalidRole
	}

	u := &User{
		Name:  in.Name,
		Email: in.Email,
		Role:  in.Role,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetByID(ctx, id)
}
