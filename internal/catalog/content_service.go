package catalog

import (
	"context"
	"errors"
	"net/url"
	"slices"

	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

const (
	maxModules          = 100
	maxLessonsPerModule = 100
	maxEditAttempts     = 3
)

var (
	ErrModuleNotFound = errs.New(errs.NotFound, "module not found")
	ErrLessonNotFound = errs.New(errs.NotFound, "lesson not found")
	ErrTooManyModules = errs.New(errs.Invalid, "a course can have at most 100 modules")
	ErrTooManyLessons = errs.New(errs.Invalid, "a module can have at most 100 lessons")
	ErrInvalidOrder   = errs.New(errs.Invalid, "the order must list every item exactly once")
	ErrLessonType     = errs.New(errs.Invalid, "unknown lesson type")
	ErrLessonContent  = errs.New(errs.Invalid,
		"articles need a body and no url; video, document and external_link lessons need an http(s) url and no body")
)

func withRevision(c *Course, r *Revision) *Course {
	out := *c
	out.Title, out.Description, out.Modules, out.UpdatedAt = r.Title, r.Description, r.Modules, r.UpdatedAt
	return &out
}

// edit applies change to the course's working copy and saves it. A concurrent edit makes the save fail,
// so change is re-applied to the fresh copy; it must therefore be safe to run more than once.
func (s *Service) edit(ctx context.Context, actor auth.Actor, id uuid.UUID, change func(*Revision) error) (*Course, error) {
	c, err := s.visible(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !canEdit(actor, c) {
		return nil, ErrNotOwner
	}

	for range maxEditAttempts {
		rev, err := s.repo.EnsureRevision(ctx, c)
		if err != nil {
			return nil, err
		}
		if err := change(rev); err != nil {
			return nil, err
		}

		err = s.repo.SaveRevision(ctx, rev)
		if errors.Is(err, ErrEditConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return withRevision(c, rev), nil
	}
	return nil, ErrEditConflict
}

func (r *Revision) moduleIndex(id uuid.UUID) int {
	return slices.IndexFunc(r.Modules, func(m Module) bool { return m.ID == id })
}

func (m *Module) lessonIndex(id uuid.UUID) int {
	return slices.IndexFunc(m.Lessons, func(l Lesson) bool { return l.ID == id })
}

// reordered returns items arranged as ids, which must be a permutation of the items' IDs.
func reordered[T any](items []T, ids []uuid.UUID, idOf func(T) uuid.UUID) ([]T, error) {
	if len(ids) != len(items) {
		return nil, ErrInvalidOrder
	}

	byID := make(map[uuid.UUID]T, len(items))
	for _, it := range items {
		byID[idOf(it)] = it
	}

	out := make([]T, 0, len(items))
	for _, id := range ids {
		it, ok := byID[id]
		if !ok {
			return nil, ErrInvalidOrder
		}
		delete(byID, id)
		out = append(out, it)
	}
	return out, nil
}

// Modules lists the content the actor sees: the working copy for editors, the published version otherwise.
func (s *Service) Modules(ctx context.Context, actor auth.Actor, courseID uuid.UUID) ([]Module, error) {
	c, err := s.GetByID(ctx, actor, courseID)
	if err != nil {
		return nil, err
	}
	return c.Modules, nil
}

func (s *Service) AddModule(ctx context.Context, actor auth.Actor, courseID uuid.UUID, title string) (Module, error) {
	var m Module
	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		if len(r.Modules) >= maxModules {
			return ErrTooManyModules
		}
		m = Module{ID: uuid.New(), Title: title}
		r.Modules = append(r.Modules, m)
		return nil
	})
	return m, err
}

func (s *Service) RenameModule(ctx context.Context, actor auth.Actor, courseID, moduleID uuid.UUID, title string) (Module, error) {
	var m Module
	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		i := r.moduleIndex(moduleID)
		if i < 0 {
			return ErrModuleNotFound
		}
		r.Modules[i].Title = title
		m = r.Modules[i]
		return nil
	})
	return m, err
}

func (s *Service) DeleteModule(ctx context.Context, actor auth.Actor, courseID, moduleID uuid.UUID) error {
	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		i := r.moduleIndex(moduleID)
		if i < 0 {
			return ErrModuleNotFound
		}
		r.Modules = slices.Delete(r.Modules, i, i+1)
		return nil
	})
	return err
}

func (s *Service) ReorderModules(ctx context.Context, actor auth.Actor, courseID uuid.UUID, ids []uuid.UUID) ([]Module, error) {
	c, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		ordered, err := reordered(r.Modules, ids, func(m Module) uuid.UUID { return m.ID })
		if err != nil {
			return err
		}
		r.Modules = ordered
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c.Modules, nil
}

type LessonInput struct {
	Title string
	Type  LessonType
	URL   string
	Body  string
}

// LessonUpdate leaves omitted fields unchanged; a lesson's type can't change.
type LessonUpdate struct {
	Title *string
	URL   *string
	Body  *string
}

func validateLesson(l Lesson) error {
	switch l.Type {
	case LessonArticle:
		if l.Body == "" || l.URL != "" {
			return ErrLessonContent
		}
	case LessonVideo, LessonDocument, LessonLink:
		if l.Body != "" || !isWebURL(l.URL) {
			return ErrLessonContent
		}
	default:
		return ErrLessonType
	}
	return nil
}

// Only http(s) is accepted so lesson links can't smuggle in javascript: or file: URLs.
func isWebURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (s *Service) AddLesson(ctx context.Context, actor auth.Actor, courseID, moduleID uuid.UUID, in LessonInput) (Lesson, error) {
	l := Lesson{Title: in.Title, Type: in.Type, URL: in.URL, Body: in.Body}
	if err := validateLesson(l); err != nil {
		return Lesson{}, err
	}

	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		i := r.moduleIndex(moduleID)
		if i < 0 {
			return ErrModuleNotFound
		}
		if len(r.Modules[i].Lessons) >= maxLessonsPerModule {
			return ErrTooManyLessons
		}
		l.ID = uuid.New()
		r.Modules[i].Lessons = append(r.Modules[i].Lessons, l)
		return nil
	})
	return l, err
}

func (s *Service) UpdateLesson(ctx context.Context, actor auth.Actor, courseID, moduleID, lessonID uuid.UUID, in LessonUpdate) (Lesson, error) {
	if in.Title == nil && in.URL == nil && in.Body == nil {
		return Lesson{}, ErrNothingToUpdate
	}

	var l Lesson
	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		mi := r.moduleIndex(moduleID)
		if mi < 0 {
			return ErrModuleNotFound
		}
		m := &r.Modules[mi]
		li := m.lessonIndex(lessonID)
		if li < 0 {
			return ErrLessonNotFound
		}

		updated := m.Lessons[li]
		if in.Title != nil {
			updated.Title = *in.Title
		}
		if in.URL != nil {
			updated.URL = *in.URL
		}
		if in.Body != nil {
			updated.Body = *in.Body
		}
		if err := validateLesson(updated); err != nil {
			return err
		}

		m.Lessons[li] = updated
		l = updated
		return nil
	})
	return l, err
}

func (s *Service) DeleteLesson(ctx context.Context, actor auth.Actor, courseID, moduleID, lessonID uuid.UUID) error {
	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		mi := r.moduleIndex(moduleID)
		if mi < 0 {
			return ErrModuleNotFound
		}
		m := &r.Modules[mi]
		li := m.lessonIndex(lessonID)
		if li < 0 {
			return ErrLessonNotFound
		}
		m.Lessons = slices.Delete(m.Lessons, li, li+1)
		return nil
	})
	return err
}

func (s *Service) ReorderLessons(ctx context.Context, actor auth.Actor, courseID, moduleID uuid.UUID, ids []uuid.UUID) (Module, error) {
	var m Module
	_, err := s.edit(ctx, actor, courseID, func(r *Revision) error {
		mi := r.moduleIndex(moduleID)
		if mi < 0 {
			return ErrModuleNotFound
		}
		ordered, err := reordered(r.Modules[mi].Lessons, ids, func(l Lesson) uuid.UUID { return l.ID })
		if err != nil {
			return err
		}
		r.Modules[mi].Lessons = ordered
		m = r.Modules[mi]
		return nil
	})
	return m, err
}
