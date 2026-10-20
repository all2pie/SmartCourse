//go:build integration

package catalog

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestMongoRepository(t *testing.T) {
	db := testutil.Mongo(t, "catalog")
	repo := NewRepository(db)
	ctx := t.Context()

	create := func(t *testing.T, instructor uuid.UUID, status Status, title string) Course {
		t.Helper()
		c := &Course{InstructorID: instructor, Title: title, Status: status}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
		// Distinct millisecond timestamps keep the newest-first order deterministic.
		time.Sleep(2 * time.Millisecond)
		return *c
	}

	t.Run("round-trip with UUIDs stored as strings", func(t *testing.T) {
		c := create(t, uuid.New(), StatusDraft, "round trip")
		got, err := repo.GetByID(ctx, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(c, *got); diff != "" {
			t.Fatalf("mismatch (-want +got):\n%s", diff)
		}

		raw, err := db.Collection("courses").FindOne(ctx, bson.D{{Key: "_id", Value: c.ID.String()}}).Raw()
		if err != nil {
			t.Fatalf("finding by string _id: %v", err)
		}
		if typ := raw.Lookup("instructor_id").Type; typ != bson.TypeString {
			t.Fatalf("instructor_id stored as %v, want string", typ)
		}
	})

	t.Run("update", func(t *testing.T) {
		c := create(t, uuid.New(), StatusDraft, "before")
		desc := "new description"
		got, err := repo.Update(ctx, c.ID, UpdateInput{Description: &desc})
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "before" || got.Description != desc || !got.UpdatedAt.After(c.UpdatedAt) {
			t.Fatalf("updated %+v", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("get: err = %v", err)
		}
		title := "x"
		if _, err := repo.Update(ctx, uuid.New(), UpdateInput{Title: &title}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("update: err = %v", err)
		}
	})

	t.Run("migrated indexes exist", func(t *testing.T) {
		specs, err := db.Collection("courses").Indexes().ListSpecifications(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, s := range specs {
			names = append(names, s.Name)
		}
		for _, want := range []string{"status_created_at", "instructor_id_created_at"} {
			if !slices.Contains(names, want) {
				t.Errorf("index %q missing; have %v", want, names)
			}
		}
	})

	t.Run("list visibility and pagination", func(t *testing.T) {
		if _, err := db.Collection("courses").DeleteMany(ctx, bson.D{}); err != nil {
			t.Fatal(err)
		}
		a, b := uuid.New(), uuid.New()
		aDraft := create(t, a, StatusDraft, "a draft")
		aPub := create(t, a, StatusPublished, "a published")
		bDraft := create(t, b, StatusDraft, "b draft")
		bPub := create(t, b, StatusPublished, "b published")

		tests := []struct {
			name string
			f    ListFilter
			want []Course
		}{
			{"admin sees all, newest first", ListFilter{}, []Course{bPub, bDraft, aPub, aDraft}},
			{"published only", ListFilter{PublishedOnly: true}, []Course{bPub, aPub}},
			{"published or own", ListFilter{PublishedOnly: true, OwnerExempt: &a}, []Course{bPub, aPub, aDraft}},
			{"another instructor's courses", ListFilter{InstructorID: &b, PublishedOnly: true, OwnerExempt: &a}, []Course{bPub}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.f.Limit = 10
				res, err := repo.List(ctx, tt.f)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(tt.want, res.Items); diff != "" || res.Total != int64(len(tt.want)) {
					t.Fatalf("total %d, items mismatch (-want +got):\n%s", res.Total, diff)
				}
			})
		}

		page, err := repo.List(ctx, ListFilter{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff([]Course{aPub, aDraft}, page.Items); diff != "" || page.Total != 4 {
			t.Fatalf("page 2: total %d, mismatch (-want +got):\n%s", page.Total, diff)
		}

		empty, err := repo.List(ctx, ListFilter{Limit: 2, Offset: 10})
		if err != nil {
			t.Fatal(err)
		}
		if empty.Items == nil || len(empty.Items) != 0 {
			t.Fatalf("past the end: items = %#v, want empty non-nil", empty.Items)
		}
	})
}
