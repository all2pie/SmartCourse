package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestPolicy(t *testing.T) {
	u := &User{}
	u.ID = uuid.New()

	if !canSeeEmail(auth.Actor{ID: u.ID, Role: auth.RoleStudent}, u) {
		t.Error("user can't see own email")
	}
	if !canSeeEmail(testutil.Actor(auth.RoleAdmin), u) {
		t.Error("admin can't see email")
	}
	if canSeeEmail(testutil.Actor(auth.RoleInstructor), u) {
		t.Error("another user sees email")
	}

	for role, want := range map[auth.Role]bool{auth.RoleStudent: true, auth.RoleInstructor: true, auth.RoleAdmin: false, "": false} {
		if got := canSelfRegisterAs(role); got != want {
			t.Errorf("canSelfRegisterAs(%q) = %v, want %v", role, got, want)
		}
	}
}

func TestRegister(t *testing.T) {
	valid := RegisterInput{Name: "Ivy", Email: "  Ivy@Example.COM ", Password: "longenough", Role: auth.RoleInstructor}

	t.Run("normalizes email and hashes password", func(t *testing.T) {
		u, err := NewService(newFakeUsers()).Register(t.Context(), valid)
		if err != nil {
			t.Fatal(err)
		}
		if u.Email != "ivy@example.com" {
			t.Errorf("email = %q", u.Email)
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(valid.Password)) != nil {
			t.Error("password hash does not match")
		}
	})

	t.Run("duplicate email after normalization", func(t *testing.T) {
		svc := NewService(newFakeUsers())
		if _, err := svc.Register(t.Context(), valid); err != nil {
			t.Fatal(err)
		}
		dup := valid
		dup.Email = "IVY@example.com"
		if _, err := svc.Register(t.Context(), dup); !errors.Is(err, ErrEmailTaken) {
			t.Fatalf("err = %v, want ErrEmailTaken", err)
		}
	})

	tests := map[string]struct {
		edit    func(*RegisterInput)
		wantErr error
	}{
		"admin role":                    {func(in *RegisterInput) { in.Role = auth.RoleAdmin }, ErrRoleNotAllowed},
		"7 characters":                  {func(in *RegisterInput) { in.Password = "1234567" }, ErrInvalidPassword},
		"73 bytes":                      {func(in *RegisterInput) { in.Password = strings.Repeat("a", 73) }, ErrInvalidPassword},
		"72 bytes":                      {func(in *RegisterInput) { in.Password = strings.Repeat("a", 72) }, nil},
		"8 multibyte characters":        {func(in *RegisterInput) { in.Password = strings.Repeat("é", 8) }, nil},
		"25 multibyte chars (75 bytes)": {func(in *RegisterInput) { in.Password = strings.Repeat("€", 25) }, ErrInvalidPassword},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := valid
			tt.edit(&in)
			if _, err := NewService(newFakeUsers()).Register(t.Context(), in); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateAdmin(t *testing.T) {
	u, err := NewService(newFakeUsers()).CreateAdmin(t.Context(), "Root", "root@example.com", "adminpassword")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != auth.RoleAdmin {
		t.Fatalf("role = %q", u.Role)
	}
}

func TestGetByIDHidesEmail(t *testing.T) {
	svc := NewService(newFakeUsers())
	u, err := svc.Register(t.Context(), RegisterInput{Name: "S", Email: "s@example.com", Password: "longenough", Role: auth.RoleStudent})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		actor     auth.Actor
		wantEmail string
	}{
		"self":       {auth.Actor{ID: u.ID, Role: auth.RoleStudent}, "s@example.com"},
		"admin":      {testutil.Actor(auth.RoleAdmin), "s@example.com"},
		"other user": {testutil.Actor(auth.RoleInstructor), ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := svc.GetByID(t.Context(), tt.actor, u.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Email != tt.wantEmail {
				t.Fatalf("email = %q, want %q", got.Email, tt.wantEmail)
			}
		})
	}

	if _, err := svc.GetByID(t.Context(), testutil.Actor(auth.RoleAdmin), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: err = %v", err)
	}
}
