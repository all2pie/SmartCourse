package catalog

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// fakeRepository is an in-memory Repository that also records the last list filter.
type fakeRepository struct {
	mu         sync.Mutex
	courses    map[uuid.UUID]Course
	lastFilter ListFilter
}

func newFakeRepository(courses ...Course) *fakeRepository {
	r := &fakeRepository{courses: map[uuid.UUID]Course{}}
	for _, c := range courses {
		r.courses[c.ID] = c
	}
	return r
}

func (r *fakeRepository) Create(_ context.Context, c *Course) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c.ID = uuid.New()
	c.CreatedAt = time.Now().UTC().Truncate(time.Millisecond)
	c.UpdatedAt = c.CreatedAt
	r.courses[c.ID] = *c
	return nil
}

func (r *fakeRepository) GetByID(_ context.Context, id uuid.UUID) (*Course, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.courses[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &c, nil
}

func (r *fakeRepository) Update(_ context.Context, id uuid.UUID, in UpdateInput) (*Course, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.courses[id]
	if !ok {
		return nil, ErrNotFound
	}
	if in.Title != nil {
		c.Title = *in.Title
	}
	if in.Description != nil {
		c.Description = *in.Description
	}
	c.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	r.courses[id] = c
	return &c, nil
}

func (r *fakeRepository) List(_ context.Context, f ListFilter) (ListResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastFilter = f

	var all []Course
	for _, c := range r.courses {
		if f.InstructorID != nil && c.InstructorID != *f.InstructorID {
			continue
		}
		if f.PublishedOnly && c.Status != StatusPublished && (f.OwnerExempt == nil || c.InstructorID != *f.OwnerExempt) {
			continue
		}
		all = append(all, c)
	}
	slices.SortFunc(all, func(a, b Course) int { return b.CreatedAt.Compare(a.CreatedAt) })

	items := []Course{}
	if f.Offset < len(all) {
		items = all[f.Offset:min(f.Offset+f.Limit, len(all))]
	}
	return ListResult{Items: items, Total: int64(len(all))}, nil
}
