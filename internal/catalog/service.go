package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

var (
	ErrNothingToUpdate = errs.New(errs.Invalid, "no fields to update")
	ErrNotInstructor   = errs.New(errs.Forbidden, "only instructors can create courses")
	ErrNotOwner        = errs.New(errs.Forbidden, "only the course's instructor or an admin can edit it")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Title       string
	Description string
}

func (s *Service) Create(ctx context.Context, actor auth.Actor, in CreateInput) (*Course, error) {
	if actor.Role != auth.RoleInstructor {
		return nil, ErrNotInstructor
	}

	c := &Course{
		InstructorID: actor.ID,
		Title:        in.Title,
		Description:  in.Description,
		Status:       StatusDraft,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// GetByID reports courses the actor may not see as not found, so their existence isn't leaked.
// Editors see their unpublished working copy; everyone else sees the published content.
func (s *Service) GetByID(ctx context.Context, actor auth.Actor, id uuid.UUID) (*Course, error) {
	c, err := s.visible(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(actor, c) {
		return c, nil
	}

	rev, err := s.repo.GetRevision(ctx, id)
	if errors.Is(err, ErrNoRevision) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	return withRevision(c, rev), nil
}

func (s *Service) visible(ctx context.Context, actor auth.Actor, id uuid.UUID) (*Course, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canView(actor, c) {
		return nil, ErrNotFound
	}
	return c, nil
}

// UpdateInput uses pointers so an omitted field is left unchanged, while an empty string clears it.
type UpdateInput struct {
	Title       *string
	Description *string
}

// Update changes the working copy; students keep seeing the published version until it is republished.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id uuid.UUID, in UpdateInput) (*Course, error) {
	if in.Title == nil && in.Description == nil {
		return nil, ErrNothingToUpdate
	}

	return s.edit(ctx, actor, id, func(r *Revision) error {
		if in.Title != nil {
			r.Title = *in.Title
		}
		if in.Description != nil {
			r.Description = *in.Description
		}
		return nil
	})
}

type ListInput struct {
	InstructorID *uuid.UUID
	Limit        int
	Offset       int
}

func (s *Service) List(ctx context.Context, actor auth.Actor, in ListInput) (ListResult, error) {
	f := ListFilter{
		InstructorID: in.InstructorID,
		Limit:        in.Limit,
		Offset:       max(in.Offset, 0),
	}
	if f.Limit <= 0 {
		f.Limit = defaultListLimit
	}
	f.Limit = min(f.Limit, maxListLimit)
	restrictToVisible(actor, &f)

	res, err := s.repo.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	res.Limit, res.Offset = f.Limit, f.Offset

	return res, nil
}
