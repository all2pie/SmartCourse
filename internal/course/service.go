package course

import (
	"context"

	"github.com/google/uuid"
)

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
