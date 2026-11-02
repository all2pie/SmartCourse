package catalog

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestHandlerContent(t *testing.T) {
	owner := testutil.Actor(auth.RoleInstructor)
	student := testutil.ActorHeaders(testutil.Actor(auth.RoleStudent))
	course := Course{ID: uuid.New(), InstructorID: owner.ID, Title: "Go", Status: StatusPublished}
	api := newTestAPI(t, course)
	base := "/courses/" + course.ID.String() + "/modules"
	as := func(extra ...any) []any { return append(testutil.ActorHeaders(owner), extra...) }

	m1 := testutil.Decode[ModuleResponse](t, api.Post(base, as(map[string]any{"title": "Basics"})...), http.StatusCreated)
	m2 := testutil.Decode[ModuleResponse](t, api.Post(base, as(map[string]any{"title": "Advanced"})...), http.StatusCreated)
	if m1.Lessons == nil || len(m1.Lessons) != 0 {
		t.Fatalf("new module lessons = %#v, want an empty array (never null)", m1.Lessons)
	}

	lessonPath := base + "/" + m1.ID.String() + "/lessons"
	article := testutil.Decode[LessonResponse](t, api.Post(lessonPath, as(map[string]any{"title": "Intro", "type": "article", "body": "hi"})...), http.StatusCreated)
	video := testutil.Decode[LessonResponse](t, api.Post(lessonPath, as(map[string]any{"title": "Demo", "type": "video", "url": "https://example.com/v"})...), http.StatusCreated)
	if article.Type != LessonArticle || article.Body != "hi" || article.URL != "" || video.URL != "https://example.com/v" {
		t.Fatalf("lessons %+v %+v", article, video)
	}

	t.Run("list shows the working copy to the owner and nothing to students", func(t *testing.T) {
		got := testutil.Decode[ModuleListResponse](t, api.Get(base, testutil.ActorHeaders(owner)...), http.StatusOK)
		if len(got.Items) != 2 || got.Items[0].ID != m1.ID || len(got.Items[0].Lessons) != 2 {
			t.Fatalf("owner list %+v", got)
		}
		got = testutil.Decode[ModuleListResponse](t, api.Get(base, student...), http.StatusOK)
		if got.Items == nil || len(got.Items) != 0 {
			t.Fatalf("student list = %#v, want an empty array", got.Items)
		}
	})

	t.Run("rename", func(t *testing.T) {
		got := testutil.Decode[ModuleResponse](t, api.Patch(base+"/"+m2.ID.String(), as(map[string]any{"title": "Deep dive"})...), http.StatusOK)
		if got.Title != "Deep dive" {
			t.Fatalf("renamed %+v", got)
		}
	})

	t.Run("reorder modules and lessons", func(t *testing.T) {
		got := testutil.Decode[ModuleListResponse](t, api.Put(base+"/order", as(map[string]any{"module_ids": []uuid.UUID{m2.ID, m1.ID}})...), http.StatusOK)
		if got.Items[0].ID != m2.ID {
			t.Fatalf("modules %+v", got)
		}
		mod := testutil.Decode[ModuleResponse](t, api.Put(lessonPath+"/order", as(map[string]any{"lesson_ids": []uuid.UUID{video.ID, article.ID}})...), http.StatusOK)
		if mod.Lessons[0].ID != video.ID {
			t.Fatalf("lessons %+v", mod.Lessons)
		}
		testutil.Problem(t, api.Put(base+"/order", as(map[string]any{"module_ids": []uuid.UUID{m1.ID}})...), http.StatusUnprocessableEntity)
	})

	t.Run("update lesson", func(t *testing.T) {
		got := testutil.Decode[LessonResponse](t, api.Patch(lessonPath+"/"+article.ID.String(), as(map[string]any{"body": "new"})...), http.StatusOK)
		if got.Body != "new" || got.Title != "Intro" {
			t.Fatalf("updated %+v", got)
		}
		testutil.Problem(t, api.Patch(lessonPath+"/"+article.ID.String(), as(map[string]any{})...), http.StatusUnprocessableEntity)
		testutil.Problem(t, api.Patch(lessonPath+"/"+article.ID.String(), as(map[string]any{"url": "https://example.com"})...), http.StatusUnprocessableEntity)
		testutil.Problem(t, api.Patch(lessonPath+"/"+uuid.NewString(), as(map[string]any{"title": "x"})...), http.StatusNotFound)
	})

	t.Run("validation", func(t *testing.T) {
		testutil.Problem(t, api.Post(base, as(map[string]any{"title": ""})...), http.StatusUnprocessableEntity)
		testutil.Problem(t, api.Post(base, as(map[string]any{"title": "x", "extra": 1})...), http.StatusUnprocessableEntity)
		testutil.Problem(t, api.Post(lessonPath, as(map[string]any{"title": "x", "type": "quiz"})...), http.StatusUnprocessableEntity)
		testutil.Problem(t, api.Post(lessonPath, as(map[string]any{"title": "x", "type": "external_link", "url": "javascript:alert(1)"})...), http.StatusUnprocessableEntity)
	})

	t.Run("students cannot edit", func(t *testing.T) {
		testutil.Problem(t, api.Post(base, append(student, map[string]any{"title": "x"})...), http.StatusForbidden)
		testutil.Problem(t, api.Delete(base+"/"+m1.ID.String(), student...), http.StatusForbidden)
	})

	t.Run("no caller is unauthenticated", func(t *testing.T) {
		testutil.Problem(t, api.Get(base), http.StatusUnauthorized)
	})

	t.Run("delete", func(t *testing.T) {
		if rec := api.Delete(lessonPath+"/"+video.ID.String(), testutil.ActorHeaders(owner)...); rec.Code != http.StatusNoContent {
			t.Fatalf("delete lesson: %d %s", rec.Code, rec.Body)
		}
		testutil.Problem(t, api.Delete(lessonPath+"/"+video.ID.String(), testutil.ActorHeaders(owner)...), http.StatusNotFound)
		if rec := api.Delete(base+"/"+m1.ID.String(), testutil.ActorHeaders(owner)...); rec.Code != http.StatusNoContent {
			t.Fatalf("delete module: %d %s", rec.Code, rec.Body)
		}
		testutil.Problem(t, api.Delete(base+"/"+m1.ID.String(), testutil.ActorHeaders(owner)...), http.StatusNotFound)
	})
}
