// Package testutil holds helpers shared by tests. Import it only from _test.go files.
package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/problem"
)

// Secret is a valid HS256 key for tests.
const Secret = "test-secret-that-is-at-least-32-bytes-long"

// Actor returns a caller with a fresh ID.
func Actor(role auth.Role) auth.Actor {
	return auth.Actor{ID: uuid.New(), Role: role}
}

// ActorHeaders returns the gateway-forwarded headers for a, in humatest's "Name: value" form.
func ActorHeaders(a auth.Actor) []any {
	return []any{auth.HeaderUserID + ": " + a.ID.String(), auth.HeaderUserRole + ": " + string(a.Role)}
}

// Tokens returns token signer/verifier keyed with Secret.
func Tokens(t testing.TB) *auth.Tokens {
	t.Helper()
	tokens, err := auth.NewTokens(Secret)
	if err != nil {
		t.Fatal(err)
	}
	return tokens
}

// Problem checks that rec holds a problem+json body with the given status and returns it.
func Problem(t testing.TB, rec *httptest.ResponseRecorder, status int) huma.ErrorModel {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type = %q, want %q", ct, problem.ContentType)
	}
	var m huma.ErrorModel
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decoding problem: %v; body: %s", err, rec.Body)
	}
	if m.Status != status || m.Title != http.StatusText(status) {
		t.Fatalf("problem status/title = %d/%q, want %d/%q", m.Status, m.Title, status, http.StatusText(status))
	}
	return m
}

// RepoRoot returns the module root: the nearest parent of the working directory holding go.mod.
func RepoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the working directory")
		}
		dir = parent
	}
}

// Decode checks rec's status and decodes its JSON body into T.
func Decode[T any](t testing.TB, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	var v T
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decoding body: %v; body: %s", err, rec.Body)
	}
	return v
}
