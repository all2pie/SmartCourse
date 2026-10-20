package catalog

import (
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func newTestAPI(t *testing.T, courses ...Course) humatest.TestAPI {
	gin.SetMode(gin.TestMode)
	api := httpapi.New(gin.New(), "Catalog", "test")
	NewHandler(NewService(newFakeRepository(courses...))).Register(api)
	return humatest.Wrap(t, api)
}

func TestHandlerCreate(t *testing.T) {
	api := newTestAPI(t)
	instructor := testutil.Actor(auth.RoleInstructor)

	rec := api.Post("/courses", append(testutil.ActorHeaders(instructor), map[string]any{"title": "Go 101"})...)
	got := testutil.Decode[CourseResponse](t, rec, http.StatusCreated)
	if got.InstructorID != instructor.ID || got.Status != StatusDraft || got.Title != "Go 101" {
		t.Fatalf("created %+v", got)
	}

	t.Run("student is forbidden", func(t *testing.T) {
		rec := api.Post("/courses", append(testutil.ActorHeaders(testutil.Actor(auth.RoleStudent)), map[string]any{"title": "x"})...)
		if p := testutil.Problem(t, rec, http.StatusForbidden); p.Detail != ErrNotInstructor.Message {
			t.Fatalf("detail = %q", p.Detail)
		}
	})

	t.Run("no forwarded caller is unauthenticated", func(t *testing.T) {
		testutil.Problem(t, api.Post("/courses", map[string]any{"title": "x"}), http.StatusUnauthorized)
	})

	t.Run("invalid role header is unauthenticated", func(t *testing.T) {
		rec := api.Post("/courses", auth.HeaderUserID+": "+instructor.ID.String(), auth.HeaderUserRole+": root", map[string]any{"title": "x"})
		testutil.Problem(t, rec, http.StatusUnauthorized)
	})

	for name, body := range map[string]any{
		"missing title": map[string]any{},
		"empty title":   map[string]any{"title": ""},
		"unknown field": map[string]any{"title": "x", "status": "published"},
	} {
		t.Run(name+" is 422", func(t *testing.T) {
			p := testutil.Problem(t, api.Post("/courses", append(testutil.ActorHeaders(instructor), body)...), http.StatusUnprocessableEntity)
			if len(p.Errors) == 0 || p.Errors[0].Location == "" {
				t.Fatalf("problem has no field errors: %+v", p)
			}
		})
	}

	t.Run("malformed JSON is 400", func(t *testing.T) {
		rec := api.Post("/courses", append(testutil.ActorHeaders(instructor), "Content-Type: application/json", strings.NewReader(`{"title":`))...)
		testutil.Problem(t, rec, http.StatusBadRequest)
	})
}

func TestHandlerGetAndUpdate(t *testing.T) {
	owner := testutil.Actor(auth.RoleInstructor)
	draft := Course{ID: uuid.New(), InstructorID: owner.ID, Title: "t", Status: StatusDraft}
	api := newTestAPI(t, draft)

	rec := api.Get("/courses/"+draft.ID.String(), testutil.ActorHeaders(owner)...)
	if got := testutil.Decode[CourseResponse](t, rec, http.StatusOK); got.ID != draft.ID {
		t.Fatalf("got %+v", got)
	}

	rec = api.Get("/courses/"+draft.ID.String(), testutil.ActorHeaders(testutil.Actor(auth.RoleStudent))...)
	testutil.Problem(t, rec, http.StatusNotFound)

	p := testutil.Problem(t, api.Get("/courses/not-a-uuid", testutil.ActorHeaders(owner)...), http.StatusUnprocessableEntity)
	if p.Errors[0].Location != "path.id" {
		t.Fatalf("location = %q", p.Errors[0].Location)
	}

	rec = api.Patch("/courses/"+draft.ID.String(), append(testutil.ActorHeaders(owner), map[string]any{})...)
	testutil.Problem(t, rec, http.StatusUnprocessableEntity)

	rec = api.Patch("/courses/"+draft.ID.String(), append(testutil.ActorHeaders(owner), map[string]any{"description": ""})...)
	if got := testutil.Decode[CourseResponse](t, rec, http.StatusOK); got.Title != "t" || got.Description != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestHandlerList(t *testing.T) {
	api := newTestAPI(t)
	student := testutil.ActorHeaders(testutil.Actor(auth.RoleStudent))

	got := testutil.Decode[map[string]any](t, api.Get("/courses", student...), http.StatusOK)
	if got["limit"] != float64(20) || got["offset"] != float64(0) {
		t.Fatalf("defaults: %v", got)
	}
	if items, ok := got["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("items = %#v, want an empty array (never null)", got["items"])
	}

	for _, q := range []string{"limit=0", "limit=101", "offset=-1", "instructor_id=nope"} {
		testutil.Problem(t, api.Get("/courses?"+q, student...), http.StatusUnprocessableEntity)
	}
}
