package identity

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

type authFixture struct {
	users  *fakeUsers
	store  *fakeTokenStore
	tokens *auth.Tokens
	auth   *AuthService
	user   *User
}

const fixturePassword = "correct-horse"

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	f := &authFixture{users: newFakeUsers(), store: newFakeTokenStore(), tokens: testutil.Tokens(t)}

	var err error
	f.auth, err = NewAuthService(f.users, f.store, f.tokens, 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	f.user, err = NewService(f.users).Register(t.Context(), RegisterInput{
		Name: "Stu", Email: "stu@example.com", Password: fixturePassword, Role: auth.RoleStudent,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *authFixture) login(t *testing.T) TokenPair {
	t.Helper()
	pair, err := f.auth.Login(t.Context(), "stu@example.com", fixturePassword)
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func TestLogin(t *testing.T) {
	f := newAuthFixture(t)

	pair, err := f.auth.Login(t.Context(), "  STU@example.com", fixturePassword)
	if err != nil {
		t.Fatal(err)
	}
	if pair.TokenType != "Bearer" || pair.ExpiresIn != 900 || pair.RefreshToken == "" {
		t.Fatalf("pair = %+v", pair)
	}
	actor, err := f.tokens.Verify(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if actor.ID != f.user.ID || actor.Role != auth.RoleStudent {
		t.Fatalf("access token actor = %+v", actor)
	}
	if _, ok := f.store.tokens[hashToken(pair.RefreshToken)]; !ok {
		t.Fatal("refresh token not stored by hash")
	}
	if _, ok := f.store.tokens[pair.RefreshToken]; ok {
		t.Fatal("refresh token stored in plain text")
	}

	for name, creds := range map[string][2]string{
		"wrong password": {"stu@example.com", "wrong-password"},
		"unknown email":  {"nobody@example.com", fixturePassword},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.auth.Login(t.Context(), creds[0], creds[1]); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	f := newAuthFixture(t)
	first := f.login(t)

	second, err := f.auth.Refresh(t.Context(), first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}

	if _, err := f.auth.Refresh(t.Context(), first.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("reusing rotated token: err = %v", err)
	}
	if _, err := f.auth.Refresh(t.Context(), second.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("current token after reuse: err = %v, want the family revoked", err)
	}

	other := f.login(t)
	if _, err := f.auth.Refresh(t.Context(), other.RefreshToken); err != nil {
		t.Fatalf("another session was affected by the revoked one: %v", err)
	}
}

func TestRefreshConcurrentOnlyOneWins(t *testing.T) {
	f := newAuthFixture(t)
	pair := f.login(t)

	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Go(func() {
			_, err := f.auth.Refresh(t.Context(), pair.RefreshToken)
			errs <- err
		})
	}
	wg.Wait()
	close(errs)

	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d of %d concurrent refreshes succeeded, want exactly 1", ok, n)
	}
}

func TestRefreshRejects(t *testing.T) {
	f := newAuthFixture(t)

	if _, err := f.auth.Refresh(t.Context(), "unknown"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("unknown token: err = %v", err)
	}

	pair := f.login(t)
	f.users.delete(f.user.ID)
	if _, err := f.auth.Refresh(t.Context(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("deleted user: err = %v", err)
	}
}

func TestLogout(t *testing.T) {
	f := newAuthFixture(t)
	pair := f.login(t)

	for range 2 {
		if err := f.auth.Logout(t.Context(), pair.RefreshToken); err != nil {
			t.Fatalf("Logout: %v", err)
		}
	}
	if _, err := f.auth.Refresh(t.Context(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("refresh after logout: err = %v", err)
	}
	if err := f.auth.Logout(t.Context(), "unknown"); err != nil {
		t.Fatalf("Logout unknown token: %v", err)
	}
}
