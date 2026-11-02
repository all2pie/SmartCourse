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
	mu        sync.Mutex
	courses   map[uuid.UUID]Course
	revisions map[uuid.UUID]Revision
	// saveConflicts makes the next n SaveRevision calls fail as if another edit won the race.
	saveConflicts int
	lastFilter    ListFilter
}

func newFakeRepository(courses ...Course) *fakeRepository {
	r := &fakeRepository{courses: map[uuid.UUID]Course{}, revisions: map[uuid.UUID]Revision{}}
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

func (r *fakeRepository) GetRevision(_ context.Context, id uuid.UUID) (*Revision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rev, ok := r.revisions[id]
	if !ok {
		return nil, ErrNoRevision
	}
	return cloneRevision(rev), nil
}

func (r *fakeRepository) EnsureRevision(_ context.Context, c *Course) (*Revision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rev, ok := r.revisions[c.ID]
	if !ok {
		now := time.Now().UTC().Truncate(time.Millisecond)
		rev = Revision{ID: c.ID, Title: c.Title, Description: c.Description, Modules: c.Modules, CreatedAt: now, UpdatedAt: now}
		r.revisions[c.ID] = rev
	}
	return cloneRevision(rev), nil
}

func (r *fakeRepository) SaveRevision(_ context.Context, rev *Revision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveConflicts > 0 {
		r.saveConflicts--
		return ErrEditConflict
	}
	if r.revisions[rev.ID].Version != rev.Version {
		return ErrEditConflict
	}

	rev.Version++
	rev.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	r.revisions[rev.ID] = *cloneRevision(*rev)
	return nil
}

// cloneRevision deep-copies the module tree so callers can't mutate stored state.
func cloneRevision(rev Revision) *Revision {
	modules := make([]Module, len(rev.Modules))
	for i, m := range rev.Modules {
		modules[i] = Module{ID: m.ID, Title: m.Title, Lessons: slices.Clone(m.Lessons)}
	}
	rev.Modules = modules
	return &rev
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
