//go:build integration

package identity

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestGormRepository(t *testing.T) {
	repo := NewRepository(testutil.Postgres(t, "identity"))
	ctx := t.Context()

	u := &User{Name: "Ivy", Email: "ivy@example.com", PasswordHash: "$2a$hash", Role: auth.RoleInstructor}
	if err := repo.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if u.ID == uuid.Nil || u.CreatedAt.IsZero() {
		t.Fatalf("database defaults not returned: %+v", u)
	}

	equateTime := cmpopts.EquateApproxTime(0)
	t.Run("get by id and email round-trip", func(t *testing.T) {
		for name, get := range map[string]func() (*User, error){
			"id":    func() (*User, error) { return repo.GetUserByID(ctx, u.ID) },
			"email": func() (*User, error) { return repo.GetUserByEmail(ctx, u.Email) },
		} {
			got, err := get()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if diff := cmp.Diff(u, got, equateTime); diff != "" {
				t.Fatalf("%s: mismatch (-want +got):\n%s", name, diff)
			}
		}
	})

	t.Run("unique email", func(t *testing.T) {
		dup := &User{Name: "Other", Email: u.Email, PasswordHash: "x", Role: auth.RoleStudent}
		if err := repo.CreateUser(ctx, dup); !errors.Is(err, ErrEmailTaken) {
			t.Fatalf("err = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.GetUserByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("by id: err = %v", err)
		}
		if _, err := repo.GetUserByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("by email: err = %v", err)
		}
	})

	t.Run("role check constraint is an internal error", func(t *testing.T) {
		err := repo.CreateUser(ctx, &User{Name: "R", Email: "r@example.com", PasswordHash: "x", Role: "root"})
		if err == nil || errs.From(err) != nil {
			t.Fatalf("err = %v, want an unclassified (500) error", err)
		}
	})

	t.Run("timestamps are UTC microseconds", func(t *testing.T) {
		if u.CreatedAt.Location() != time.UTC || u.CreatedAt.Nanosecond()%1000 != 0 {
			t.Fatalf("created_at = %v", u.CreatedAt)
		}
	})
}
