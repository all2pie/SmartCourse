package catalog

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func moduleTitles(ms []Module) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Title)
	}
	return out
}

func moduleIDs(ms []Module) []uuid.UUID {
	var out []uuid.UUID
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func TestServiceModuleLifecycle(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	student := testutil.Actor(auth.RoleStudent)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Title: "Go", Status: StatusPublished}
	svc := NewService(newFakeRepository(course))

	a, err := svc.AddModule(ctx, owner, course.ID, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.AddModule(ctx, owner, course.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == uuid.Nil || a.ID == b.ID {
		t.Fatalf("module IDs %v, %v", a.ID, b.ID)
	}

	if _, err := svc.RenameModule(ctx, owner, course.ID, a.ID, "A2"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ReorderModules(ctx, owner, course.ID, []uuid.UUID{b.ID, a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"B", "A2"}, moduleTitles(got)); diff != "" {
		t.Fatalf("after reorder (-want +got):\n%s", diff)
	}

	t.Run("editors see the working copy, students the published content", func(t *testing.T) {
		if ms, err := svc.Modules(ctx, owner, course.ID); err != nil || len(ms) != 2 {
			t.Fatalf("owner modules = %v, %v", ms, err)
		}
		if ms, err := svc.Modules(ctx, student, course.ID); err != nil || len(ms) != 0 {
			t.Fatalf("student modules = %v, %v; want the unchanged published (empty) content", ms, err)
		}
		if c, err := svc.GetByID(ctx, student, course.ID); err != nil || c.Title != "Go" {
			t.Fatalf("student course = %+v, %v", c, err)
		}
	})

	if err := svc.DeleteModule(ctx, owner, course.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	ms, err := svc.Modules(ctx, owner, course.ID)
	if err != nil || len(ms) != 1 || ms[0].ID != a.ID {
		t.Fatalf("after delete: %v, %v", ms, err)
	}
	if err := svc.DeleteModule(ctx, owner, course.ID, b.ID); !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("deleting twice: err = %v, want ErrModuleNotFound", err)
	}
	if _, err := svc.RenameModule(ctx, owner, course.ID, uuid.New(), "x"); !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("renaming unknown: err = %v, want ErrModuleNotFound", err)
	}
}

func TestServiceEditAuthorization(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	draft := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}
	published := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusPublished}
	svc := NewService(newFakeRepository(draft, published))

	tests := []struct {
		name    string
		actor   auth.Actor
		id      uuid.UUID
		wantErr error
	}{
		{"owner", owner, draft.ID, nil},
		{"admin", testutil.Actor(auth.RoleAdmin), draft.ID, nil},
		{"other instructor can't see the draft", testutil.Actor(auth.RoleInstructor), draft.ID, ErrNotFound},
		{"student can't see the draft", testutil.Actor(auth.RoleStudent), draft.ID, ErrNotFound},
		{"student sees but can't edit a published course", testutil.Actor(auth.RoleStudent), published.ID, ErrNotOwner},
		{"other instructor sees but can't edit a published course", testutil.Actor(auth.RoleInstructor), published.ID, ErrNotOwner},
		{"unknown course", owner, uuid.New(), ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.AddModule(ctx, tt.actor, tt.id, "m"); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceUpdateKeepsPublishedVersion(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Title: "live", Description: "d", Status: StatusPublished}
	svc := NewService(newFakeRepository(course))

	if _, err := svc.Update(ctx, owner, course.ID, UpdateInput{Title: ptr("draft")}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetByID(ctx, owner, course.ID)
	if err != nil || got.Title != "draft" || got.Description != "d" {
		t.Fatalf("owner sees %+v, %v", got, err)
	}
	got, err = svc.GetByID(ctx, testutil.Actor(auth.RoleStudent), course.ID)
	if err != nil || got.Title != "live" {
		t.Fatalf("student sees %+v, %v", got, err)
	}
}

func TestServiceReorderModulesRejectsBadOrders(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}
	svc := NewService(newFakeRepository(course))

	a, _ := svc.AddModule(ctx, owner, course.ID, "A")
	b, _ := svc.AddModule(ctx, owner, course.ID, "B")

	for name, ids := range map[string][]uuid.UUID{
		"missing one": {a.ID},
		"duplicate":   {a.ID, a.ID},
		"unknown":     {a.ID, uuid.New()},
		"extra":       {a.ID, b.ID, uuid.New()},
		"empty":       {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.ReorderModules(ctx, owner, course.ID, ids); !errors.Is(err, ErrInvalidOrder) {
				t.Fatalf("err = %v, want ErrInvalidOrder", err)
			}
		})
	}

	ms, _ := svc.Modules(ctx, owner, course.ID)
	if diff := cmp.Diff([]uuid.UUID{a.ID, b.ID}, moduleIDs(ms)); diff != "" {
		t.Fatalf("order changed by rejected requests (-want +got):\n%s", diff)
	}
}

func TestServiceModuleLimit(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}
	svc := NewService(newFakeRepository(course))

	for range maxModules {
		if _, err := svc.AddModule(ctx, owner, course.ID, "m"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.AddModule(ctx, owner, course.ID, "one too many"); !errors.Is(err, ErrTooManyModules) {
		t.Fatalf("err = %v, want ErrTooManyModules", err)
	}
}

func TestServiceEditRetriesOnConflict(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}

	t.Run("a lost race is retried", func(t *testing.T) {
		repo := newFakeRepository(course)
		repo.saveConflicts = maxEditAttempts - 1
		if _, err := NewService(repo).AddModule(ctx, owner, course.ID, "m"); err != nil {
			t.Fatal(err)
		}
		if ms, _ := NewService(repo).Modules(ctx, owner, course.ID); len(ms) != 1 {
			t.Fatalf("modules = %d, want exactly one (retries must not duplicate)", len(ms))
		}
	})

	t.Run("giving up reports a conflict", func(t *testing.T) {
		repo := newFakeRepository(course)
		repo.saveConflicts = maxEditAttempts
		if _, err := NewService(repo).AddModule(ctx, owner, course.ID, "m"); !errors.Is(err, ErrEditConflict) {
			t.Fatalf("err = %v, want ErrEditConflict", err)
		}
	})
}

func TestServiceLessons(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}
	svc := NewService(newFakeRepository(course))
	m, err := svc.AddModule(ctx, owner, course.ID, "M")
	if err != nil {
		t.Fatal(err)
	}

	valid := []LessonInput{
		{Title: "v", Type: LessonVideo, URL: "https://example.com/v.mp4"},
		{Title: "d", Type: LessonDocument, URL: "http://example.com/d.pdf"},
		{Title: "l", Type: LessonLink, URL: "https://example.com"},
		{Title: "a", Type: LessonArticle, Body: "# Hello"},
	}
	var added []Lesson
	for _, in := range valid {
		l, err := svc.AddLesson(ctx, owner, course.ID, m.ID, in)
		if err != nil {
			t.Fatalf("%s: %v", in.Type, err)
		}
		added = append(added, l)
	}

	invalid := map[string]LessonInput{
		"video without url":                {Title: "x", Type: LessonVideo},
		"video with body":                  {Title: "x", Type: LessonVideo, URL: "https://example.com", Body: "b"},
		"javascript url":                   {Title: "x", Type: LessonLink, URL: "javascript:alert(1)"},
		"file url":                         {Title: "x", Type: LessonDocument, URL: "file:///etc/passwd"},
		"relative url":                     {Title: "x", Type: LessonVideo, URL: "/v.mp4"},
		"url without host":                 {Title: "x", Type: LessonVideo, URL: "https://"},
		"article without body":             {Title: "x", Type: LessonArticle},
		"article with url":                 {Title: "x", Type: LessonArticle, Body: "b", URL: "https://example.com"},
		"unknown type":                     {Title: "x", Type: "quiz", Body: "b"},
		"article with only whitespace url": {Title: "x", Type: LessonArticle, Body: "b", URL: " "},
	}
	for name, in := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := svc.AddLesson(ctx, owner, course.ID, m.ID, in)
			if !errors.Is(err, ErrLessonContent) && !errors.Is(err, ErrLessonType) {
				t.Fatalf("err = %v, want a lesson validation error", err)
			}
		})
	}

	t.Run("update merges and revalidates", func(t *testing.T) {
		video := added[0]
		l, err := svc.UpdateLesson(ctx, owner, course.ID, m.ID, video.ID, LessonUpdate{Title: ptr("renamed")})
		if err != nil || l.Title != "renamed" || l.URL != video.URL || l.Type != LessonVideo {
			t.Fatalf("updated %+v, %v", l, err)
		}
		if _, err := svc.UpdateLesson(ctx, owner, course.ID, m.ID, video.ID, LessonUpdate{Body: ptr("text")}); !errors.Is(err, ErrLessonContent) {
			t.Fatalf("adding a body to a video: err = %v, want ErrLessonContent", err)
		}
		if _, err := svc.UpdateLesson(ctx, owner, course.ID, m.ID, video.ID, LessonUpdate{}); !errors.Is(err, ErrNothingToUpdate) {
			t.Fatalf("empty update: err = %v, want ErrNothingToUpdate", err)
		}
		if _, err := svc.UpdateLesson(ctx, owner, course.ID, m.ID, uuid.New(), LessonUpdate{Title: ptr("x")}); !errors.Is(err, ErrLessonNotFound) {
			t.Fatalf("unknown lesson: err = %v, want ErrLessonNotFound", err)
		}
		if _, err := svc.UpdateLesson(ctx, owner, course.ID, uuid.New(), video.ID, LessonUpdate{Title: ptr("x")}); !errors.Is(err, ErrModuleNotFound) {
			t.Fatalf("unknown module: err = %v, want ErrModuleNotFound", err)
		}
	})

	t.Run("reorder and delete", func(t *testing.T) {
		ids := make([]uuid.UUID, 0, len(added))
		for _, l := range added {
			ids = append(ids, l.ID)
		}
		slices.Reverse(ids)

		got, err := svc.ReorderLessons(ctx, owner, course.ID, m.ID, ids)
		if err != nil {
			t.Fatal(err)
		}
		var gotIDs []uuid.UUID
		for _, l := range got.Lessons {
			gotIDs = append(gotIDs, l.ID)
		}
		if diff := cmp.Diff(ids, gotIDs); diff != "" {
			t.Fatalf("lesson order (-want +got):\n%s", diff)
		}
		if _, err := svc.ReorderLessons(ctx, owner, course.ID, m.ID, ids[1:]); !errors.Is(err, ErrInvalidOrder) {
			t.Fatalf("partial order: err = %v, want ErrInvalidOrder", err)
		}

		if err := svc.DeleteLesson(ctx, owner, course.ID, m.ID, ids[0]); err != nil {
			t.Fatal(err)
		}
		if err := svc.DeleteLesson(ctx, owner, course.ID, m.ID, ids[0]); !errors.Is(err, ErrLessonNotFound) {
			t.Fatalf("deleting twice: err = %v, want ErrLessonNotFound", err)
		}
	})

	t.Run("deleting a module deletes its lessons", func(t *testing.T) {
		if err := svc.DeleteModule(ctx, owner, course.ID, m.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.AddLesson(ctx, owner, course.ID, m.ID, valid[0]); !errors.Is(err, ErrModuleNotFound) {
			t.Fatalf("err = %v, want ErrModuleNotFound", err)
		}
	})
}

func TestServiceLessonLimit(t *testing.T) {
	ctx := t.Context()
	owner := testutil.Actor(auth.RoleInstructor)
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}
	svc := NewService(newFakeRepository(course))
	m, _ := svc.AddModule(ctx, owner, course.ID, "M")

	in := LessonInput{Title: "a", Type: LessonArticle, Body: strings.Repeat("x", 10)}
	for range maxLessonsPerModule {
		if _, err := svc.AddLesson(ctx, owner, course.ID, m.ID, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.AddLesson(ctx, owner, course.ID, m.ID, in); !errors.Is(err, ErrTooManyLessons) {
		t.Fatalf("err = %v, want ErrTooManyLessons", err)
	}
}
