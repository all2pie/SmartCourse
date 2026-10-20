package identity

import (
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func newTestAPI(t *testing.T) (humatest.TestAPI, *authFixture) {
	f := newAuthFixture(t)
	gin.SetMode(gin.TestMode)
	api := httpapi.New(gin.New(), "Identity", "test")
	NewHandler(NewService(f.users), f.auth).Register(api)
	return humatest.Wrap(t, api), f
}

func TestHandlerRegister(t *testing.T) {
	api, _ := newTestAPI(t)
	body := map[string]any{"name": "Ivy", "email": "ivy@example.com", "password": "longenough", "role": "instructor"}

	rec := api.Post("/auth/register", body)
	got := testutil.Decode[map[string]any](t, rec, http.StatusCreated)
	if got["email"] != "ivy@example.com" || got["role"] != "instructor" {
		t.Fatalf("got %v", got)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("response leaks the password: %s", rec.Body)
	}

	testutil.Problem(t, api.Post("/auth/register", body), http.StatusConflict)

	for name, edit := range map[string]func(map[string]any){
		"admin role":     func(b map[string]any) { b["role"] = "admin" },
		"short password": func(b map[string]any) { b["password"] = "short" },
		"bad email":      func(b map[string]any) { b["email"] = "not-an-email" },
		"empty name":     func(b map[string]any) { b["name"] = "" },
		"unknown field":  func(b map[string]any) { b["is_admin"] = true },
	} {
		t.Run(name, func(t *testing.T) {
			b := map[string]any{"name": "X", "email": "x@example.com", "password": "longenough", "role": "student"}
			edit(b)
			testutil.Problem(t, api.Post("/auth/register", b), http.StatusUnprocessableEntity)
		})
	}
}

func TestHandlerErrorsDoNotEchoSecrets(t *testing.T) {
	api, _ := newTestAPI(t)
	const secret = "s3cr3t!"

	rec := api.Post("/auth/register", map[string]any{"name": "X", "email": "x@example.com", "password": secret, "role": "student"})
	if p := testutil.Problem(t, rec, http.StatusUnprocessableEntity); len(p.Errors) == 0 || p.Errors[0].Location != "body.password" {
		t.Fatalf("problem = %+v", p)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("422 echoes the password: %s", rec.Body)
	}

	rec = api.Post("/auth/login", "Content-Type: application/json", strings.NewReader(`{"email":"x@example.com","password":"`+secret+`"`))
	testutil.Problem(t, rec, http.StatusBadRequest)
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("400 echoes the raw body: %s", rec.Body)
	}
}

func TestHandlerSession(t *testing.T) {
	api, f := newTestAPI(t)

	rec := api.Post("/auth/login", map[string]any{"email": "stu@example.com", "password": fixturePassword})
	tokens := testutil.Decode[TokenResponse](t, rec, http.StatusOK)
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	p := testutil.Problem(t, api.Post("/auth/login", map[string]any{"email": "stu@example.com", "password": "nope"}), http.StatusUnauthorized)
	if p.Detail != ErrInvalidCredentials.Message {
		t.Fatalf("detail = %q", p.Detail)
	}

	testutil.Problem(t, api.Post("/auth/refresh", map[string]any{}), http.StatusUnprocessableEntity)

	rec = api.Post("/auth/refresh", map[string]any{"refresh_token": tokens.RefreshToken})
	rotated := testutil.Decode[TokenResponse](t, rec, http.StatusOK)

	if rec := api.Post("/auth/logout", map[string]any{"refresh_token": rotated.RefreshToken}); rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("logout = %d %q", rec.Code, rec.Body)
	}
	testutil.Problem(t, api.Post("/auth/refresh", map[string]any{"refresh_token": rotated.RefreshToken}), http.StatusUnauthorized)

	me := testutil.Decode[UserResponse](t, api.Get("/users/me", testutil.ActorHeaders(auth.Actor{ID: f.user.ID, Role: auth.RoleStudent})...), http.StatusOK)
	if me.Email != "stu@example.com" {
		t.Fatalf("me = %+v", me)
	}

	other := testutil.Decode[map[string]any](t, api.Get("/users/"+f.user.ID.String(), testutil.ActorHeaders(testutil.Actor(auth.RoleInstructor))...), http.StatusOK)
	if _, ok := other["email"]; ok {
		t.Fatalf("email visible to another user: %v", other)
	}

	testutil.Problem(t, api.Get("/users/me"), http.StatusUnauthorized)
}
