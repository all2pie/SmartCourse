package catalog

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func ptr[T any](v T) *T { return &v }

func TestServiceCreate(t *testing.T) {
	ctx := t.Context()

	for _, role := range []auth.Role{auth.RoleStudent, auth.RoleAdmin} {
		t.Run(string(role)+" is rejected", func(t *testing.T) {
			_, err := NewService(newFakeRepository()).Create(ctx, testutil.Actor(role), CreateInput{Title: "x"})
			if !errors.Is(err, ErrNotInstructor) {
				t.Fatalf("err = %v, want ErrNotInstructor", err)
			}
		})
	}

	t.Run("instructor owns a new draft", func(t *testing.T) {
		instructor := testutil.Actor(auth.RoleInstructor)
		c, err := NewService(newFakeRepository()).Create(ctx, instructor, CreateInput{Title: "Go 101", Description: "d"})
		if err != nil {
			t.Fatal(err)
		}
		if c.InstructorID != instructor.ID || c.Status != StatusDraft || c.Title != "Go 101" || c.ID == uuid.Nil {
			t.Fatalf("created %+v", c)
		}
	})
}

func TestServiceGetByIDHidesInvisible(t *testing.T) {
	owner := testutil.Actor(auth.RoleInstructor)
	draft := Course{ID: uuid.New(), InstructorID: owner.ID, Status: StatusDraft}
	svc := NewService(newFakeRepository(draft))

	for _, a := range []auth.Actor{testutil.Actor(auth.RoleStudent), testutil.Actor(auth.RoleInstructor)} {
		if _, err := svc.GetByID(t.Context(), a, draft.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", a.Role, err)
		}
	}
	for _, a := range []auth.Actor{owner, testutil.Actor(auth.RoleAdmin)} {
		if _, err := svc.GetByID(t.Context(), a, draft.ID); err != nil {
			t.Errorf("%s: err = %v", a.Role, err)
		}
	}
	if _, err := svc.GetByID(t.Context(), owner, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing course: err = %v, want ErrNotFound", err)
	}
}

func TestServiceUpdate(t *testing.T) {
	owner := testutil.Actor(auth.RoleInstructor)
	draft := Course{ID: uuid.New(), InstructorID: owner.ID, Title: "old", Description: "keep", Status: StatusDraft}
	published := Course{ID: uuid.New(), InstructorID: owner.ID, Title: "pub", Status: StatusPublished}

	tests := []struct {
		name    string
		actor   auth.Actor
		id      uuid.UUID
		in      UpdateInput
		wantErr error
	}{
		{"no fields", owner, draft.ID, UpdateInput{}, ErrNothingToUpdate},
		{"invisible draft is not found", testutil.Actor(auth.RoleInstructor), draft.ID, UpdateInput{Title: ptr("x")}, ErrNotFound},
		{"visible but not owner is forbidden", testutil.Actor(auth.RoleStudent), published.ID, UpdateInput{Title: ptr("x")}, ErrNotOwner},
		{"owner", owner, draft.ID, UpdateInput{Title: ptr("new")}, nil},
		{"admin", testutil.Actor(auth.RoleAdmin), draft.ID, UpdateInput{Title: ptr("new")}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewService(newFakeRepository(draft, published)).Update(t.Context(), tt.actor, tt.id, tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (c.Title != "new" || c.Description != "keep") {
				t.Fatalf("updated %+v, want title changed and description kept", c)
			}
		})
	}
}

func TestServiceListAppliesDefaultsAndVisibility(t *testing.T) {
	instructor := testutil.Actor(auth.RoleInstructor)
	other := uuid.New()

	tests := []struct {
		name  string
		actor auth.Actor
		in    ListInput
		want  ListFilter
	}{
		{"defaults", testutil.Actor(auth.RoleAdmin), ListInput{}, ListFilter{Limit: 20}},
		{"limit clamped", testutil.Actor(auth.RoleAdmin), ListInput{Limit: 500, Offset: -3}, ListFilter{Limit: 100}},
		{"student", testutil.Actor(auth.RoleStudent), ListInput{Limit: 5, Offset: 10}, ListFilter{PublishedOnly: true, Limit: 5, Offset: 10}},
		{
			"instructor filtering by another instructor", instructor, ListInput{InstructorID: &other},
			ListFilter{InstructorID: &other, PublishedOnly: true, OwnerExempt: &instructor.ID, Limit: 20},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			res, err := NewService(repo).List(t.Context(), tt.actor, tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, repo.lastFilter); diff != "" {
				t.Fatalf("filter mismatch (-want +got):\n%s", diff)
			}
			if res.Limit != tt.want.Limit || res.Offset != tt.want.Offset || res.Items == nil {
				t.Fatalf("result limit/offset/items = %d/%d/%v", res.Limit, res.Offset, res.Items)
			}
		})
	}
}
