//go:build integration

package catalog

import (
	"errors"
	"slices"
	"sync"
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

	t.Run("revision lifecycle", func(t *testing.T) {
		c := create(t, uuid.New(), StatusPublished, "live")
		c.Modules = []Module{{ID: uuid.New(), Title: "m1", Lessons: []Lesson{{ID: uuid.New(), Title: "l1", Type: LessonVideo, URL: "https://example.com/v"}}}}

		if _, err := repo.GetRevision(ctx, c.ID); !errors.Is(err, ErrNoRevision) {
			t.Fatalf("before any edit: err = %v, want ErrNoRevision", err)
		}

		rev, err := repo.EnsureRevision(ctx, &c)
		if err != nil {
			t.Fatal(err)
		}
		if rev.ID != c.ID || rev.Title != "live" || rev.Version != 0 || cmp.Diff(c.Modules, rev.Modules) != "" {
			t.Fatalf("ensured %+v", rev)
		}

		rev.Title = "draft"
		stale := *rev
		if err := repo.SaveRevision(ctx, rev); err != nil {
			t.Fatal(err)
		}
		if rev.Version != 1 {
			t.Fatalf("version = %d, want 1", rev.Version)
		}
		if err := repo.SaveRevision(ctx, &stale); !errors.Is(err, ErrEditConflict) {
			t.Fatalf("stale save: err = %v, want ErrEditConflict", err)
		}

		// A second Ensure returns the stored draft instead of resetting it from the course.
		again, err := repo.EnsureRevision(ctx, &c)
		if err != nil {
			t.Fatal(err)
		}
		if again.Title != "draft" || again.Version != 1 {
			t.Fatalf("ensured again %+v", again)
		}

		got, err := repo.GetRevision(ctx, c.ID)
		if err != nil || got.Title != "draft" {
			t.Fatalf("get = %+v, %v", got, err)
		}
	})

	t.Run("concurrent first edits create one revision", func(t *testing.T) {
		c := create(t, uuid.New(), StatusDraft, "race")

		var wg sync.WaitGroup
		errCh := make(chan error, 8)
		for range cap(errCh) {
			wg.Go(func() {
				_, err := repo.EnsureRevision(ctx, &c)
				errCh <- err
			})
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatal(err)
			}
		}

		n, err := db.Collection("course_revisions").CountDocuments(ctx, bson.D{{Key: "_id", Value: c.ID}})
		if err != nil || n != 1 {
			t.Fatalf("revisions = %d, %v", n, err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("get: err = %v", err)
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
