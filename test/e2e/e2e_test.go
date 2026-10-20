//go:build e2e

// Package e2e drives the running stack (`task up`) through the gateway, like a client would,
// and validates every response of a documented operation against api/openapi.yaml.
package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

type client struct {
	t         *testing.T
	base      string
	router    routers.Router
	validated atomic.Int32
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		t.Fatalf("decoding %s: %v", r.body, err)
	}
	return m
}

func (r response) str(t *testing.T, key string) string {
	t.Helper()
	s, _ := r.json(t)[key].(string)
	return s
}

func newClient(t *testing.T) *client {
	base := os.Getenv("E2E_BASE_URL")
	if base == "" {
		base = "http://localhost:8000"
	}

	spec, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil {
		t.Fatalf("loading spec: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("invalid spec: %v", err)
	}
	doc.Servers = openapi3.Servers{{URL: base}}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}

	c := &client{t: t, base: base, router: router}
	served := c.do(http.MethodGet, "/openapi.yaml", "", nil)
	if served.status != http.StatusOK || !bytes.Equal(served.body, spec) {
		t.Fatal("the gateway serves a different spec than api/openapi.yaml; rebuild with `task up`")
	}
	return c
}

// do sends a request; body may be nil, a string (sent as is) or a value encoded as JSON.
func (c *client) do(method, path, token string, body any, headers ...string) response {
	c.t.Helper()
	var raw []byte
	switch b := body.(type) {
	case nil:
	case string:
		raw = []byte(b)
	default:
		var err error
		if raw, err = json.Marshal(b); err != nil {
			c.t.Fatal(err)
		}
	}

	req, err := http.NewRequestWithContext(c.t.Context(), method, c.base+path, bytes.NewReader(raw))
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, h := range headers {
		k, v, _ := strings.Cut(h, ": ")
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		c.t.Fatal(err)
	}

	r := response{status: res.StatusCode, header: res.Header, body: out}
	c.validate(req, raw, r)
	return r
}

// validate checks a response against its documented operation; undocumented routes are skipped.
func (c *client) validate(req *http.Request, body []byte, r response) {
	c.t.Helper()
	route, params, err := c.router.FindRoute(req)
	if err != nil {
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 r.status,
		Header:                 r.header,
		Body:                   io.NopCloser(bytes.NewReader(r.body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(c.t.Context(), in); err != nil {
		c.t.Errorf("%s %s -> %d does not match the spec: %v\nbody: %s", req.Method, req.URL.Path, r.status, err, r.body)
		return
	}
	c.validated.Add(1)
}

func expect(t *testing.T, r response, status int, what string) {
	t.Helper()
	if r.status != status {
		t.Errorf("%s: status %d, want %d; body: %s", what, r.status, status, r.body)
	}
}

func expectProblem(t *testing.T, r response, status int, what string) {
	t.Helper()
	expect(t, r, status, what)
	if ct := r.header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("%s: Content-Type %q, want application/problem+json", what, ct)
	}
}

func TestEndToEnd(t *testing.T) {
	c := newClient(t)
	run := rand.N(1 << 30)
	studentEmail := fmt.Sprintf("stu%d@example.com", run)
	instructorEmail := fmt.Sprintf("ivy%d@example.com", run)
	register := func(name, email, password, role string) response {
		return c.do(http.MethodPost, "/auth/register", "", map[string]any{"name": name, "email": email, "password": password, "role": role})
	}
	login := func(email, password string) response {
		return c.do(http.MethodPost, "/auth/login", "", map[string]any{"email": email, "password": password})
	}

	// Registration.
	r := register("Stu", studentEmail, "studentpass1", "student")
	expect(t, r, http.StatusCreated, "register student")
	if bytes.Contains(r.body, []byte("password")) {
		t.Errorf("register response leaks the password: %s", r.body)
	}
	studentID := r.str(t, "id")

	r = register("Ivy", strings.ToUpper(instructorEmail[:3])+instructorEmail[3:], "instructorpass1", "instructor")
	expect(t, r, http.StatusCreated, "register instructor")
	if got := r.str(t, "email"); got != instructorEmail {
		t.Errorf("stored email %q, want normalized %q", got, instructorEmail)
	}
	instructorID := r.str(t, "id")

	expectProblem(t, register("A", fmt.Sprintf("adm%d@example.com", run), "longpassword1", "admin"), http.StatusUnprocessableEntity, "self-register as admin")
	r = register("X", fmt.Sprintf("x%d@example.com", run), "s3cr3t!", "student")
	expectProblem(t, r, http.StatusUnprocessableEntity, "short password")
	if bytes.Contains(r.body, []byte("s3cr3t!")) {
		t.Errorf("validation error echoes the password: %s", r.body)
	}
	expectProblem(t, register("X", fmt.Sprintf("x%d@example.com", run), strings.Repeat("a", 73), "student"), http.StatusUnprocessableEntity, "73-byte password")
	expectProblem(t, register("Ivy", instructorEmail, "instructorpass1", "instructor"), http.StatusConflict, "duplicate email")

	// Login.
	wrong := login(instructorEmail, "wrong-password")
	unknown := login("nobody"+instructorEmail, "whatever123")
	expectProblem(t, wrong, http.StatusUnauthorized, "wrong password")
	expectProblem(t, unknown, http.StatusUnauthorized, "unknown email")
	if wrong.str(t, "detail") != unknown.str(t, "detail") {
		t.Errorf("wrong password and unknown email differ: %q vs %q", wrong.body, unknown.body)
	}

	r = login(strings.ToUpper(instructorEmail), "instructorpass1")
	expect(t, r, http.StatusOK, "instructor login")
	if cc := r.header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("token response Cache-Control %q", cc)
	}
	instructorAT, instructorRT := r.str(t, "access_token"), r.str(t, "refresh_token")
	r = login(studentEmail, "studentpass1")
	expect(t, r, http.StatusOK, "student login")
	studentAT := r.str(t, "access_token")
	if instructorAT == "" || studentAT == "" {
		t.Fatal("no access tokens; later steps depend on them")
	}

	t.Run("access token claims", func(t *testing.T) {
		payload, err := base64.RawURLEncoding.DecodeString(strings.Split(instructorAT, ".")[1])
		if err != nil {
			t.Fatal(err)
		}
		var claims map[string]any
		if err := json.Unmarshal(payload, &claims); err != nil {
			t.Fatal(err)
		}
		keys := slices.Sorted(maps.Keys(claims))
		if !slices.Equal(keys, []string{"exp", "iat", "iss", "role", "sub"}) {
			t.Fatalf("claims %v", keys)
		}
	})

	// Gateway authentication.
	r = c.do(http.MethodGet, "/courses", "", nil)
	expectProblem(t, r, http.StatusUnauthorized, "no token")
	if r.header.Get("WWW-Authenticate") == "" {
		t.Error("401 without WWW-Authenticate")
	}
	expectProblem(t, c.do(http.MethodGet, "/courses", "garbage", nil), http.StatusUnauthorized, "garbage token")
	expectProblem(t, c.do(http.MethodGet, "/courses", instructorAT[:len(instructorAT)-1]+"x", nil), http.StatusUnauthorized, "tampered signature")
	expectProblem(t, c.do(http.MethodGet, "/courses", "", nil, "Authorization: Basic YTpi"), http.StatusUnauthorized, "basic scheme")
	expect(t, c.do(http.MethodGet, "/users/me", "", nil, "Authorization: bearer "+instructorAT), http.StatusOK, "lowercase bearer")
	expectProblem(t, c.do(http.MethodGet, "/nope", "", nil), http.StatusNotFound, "unknown route")

	// The stack runs with APP_ENV=development, so the self-hosted docs UI is served.
	for _, path := range []string{"/docs", "/docs/assets/swagger-ui-bundle.js", "/docs/assets/swagger-ui.css", "/docs/assets/init.js"} {
		r := c.do(http.MethodGet, path, "", nil)
		expect(t, r, http.StatusOK, path)
		if !strings.HasPrefix(r.header.Get("Content-Security-Policy"), "default-src 'self';") {
			t.Errorf("%s: Content-Security-Policy %q", path, r.header.Get("Content-Security-Policy"))
		}
	}

	// Users.
	r = c.do(http.MethodGet, "/users/"+studentID, instructorAT, nil)
	expect(t, r, http.StatusOK, "another user's profile")
	if _, ok := r.json(t)["email"]; ok {
		t.Error("email visible to another user")
	}
	r = c.do(http.MethodGet, "/users/me", studentAT, nil, "X-User-ID: "+instructorID, "X-User-Role: admin")
	expect(t, r, http.StatusOK, "spoofed identity headers")
	if r.str(t, "id") != studentID || r.str(t, "role") != "student" {
		t.Errorf("spoofed headers were trusted: %s", r.body)
	}
	expectProblem(t, c.do(http.MethodGet, "/users/"+uuid.NewString(), instructorAT, nil), http.StatusNotFound, "unknown user")

	// Courses.
	r = c.do(http.MethodPost, "/courses", instructorAT, map[string]any{"title": "Go 101"})
	expect(t, r, http.StatusCreated, "create course")
	courseID := r.str(t, "id")
	if r.str(t, "instructor_id") != instructorID || r.str(t, "status") != "draft" {
		t.Errorf("created course %s", r.body)
	}
	expectProblem(t, c.do(http.MethodPost, "/courses", studentAT, map[string]any{"title": "x"}), http.StatusForbidden, "student creates")
	expectProblem(t, c.do(http.MethodPost, "/courses", studentAT, map[string]any{"title": "x"}, "X-User-Role: instructor"), http.StatusForbidden, "student creates with spoofed role")
	expectProblem(t, c.do(http.MethodPost, "/courses", instructorAT, map[string]any{"title": "x", "status": "published"}), http.StatusUnprocessableEntity, "unknown field")
	expectProblem(t, c.do(http.MethodPost, "/courses", instructorAT, `{"title":`), http.StatusBadRequest, "malformed JSON")
	expectProblem(t, c.do(http.MethodGet, "/courses/not-a-uuid", instructorAT, nil), http.StatusUnprocessableEntity, "bad course id")
	expectProblem(t, c.do(http.MethodGet, "/courses?limit=500", instructorAT, nil), http.StatusUnprocessableEntity, "limit too large")
	expectProblem(t, c.do(http.MethodDelete, "/courses/"+courseID, instructorAT, nil), http.StatusMethodNotAllowed, "unsupported method")

	expectProblem(t, c.do(http.MethodGet, "/courses/"+courseID, studentAT, nil), http.StatusNotFound, "student reads a draft")
	expect(t, c.do(http.MethodGet, "/courses/"+courseID, instructorAT, nil), http.StatusOK, "owner reads the draft")
	expectProblem(t, c.do(http.MethodPatch, "/courses/"+courseID, instructorAT, map[string]any{}), http.StatusUnprocessableEntity, "empty patch")
	r = c.do(http.MethodPatch, "/courses/"+courseID, instructorAT, map[string]any{"description": "Learn Go"})
	expect(t, r, http.StatusOK, "patch description")
	if r.str(t, "title") != "Go 101" || r.str(t, "description") != "Learn Go" {
		t.Errorf("patched course %s", r.body)
	}

	r = c.do(http.MethodGet, "/courses?instructor_id="+instructorID, instructorAT, nil)
	expect(t, r, http.StatusOK, "instructor lists own courses")
	if r.json(t)["total"] != float64(1) {
		t.Errorf("instructor's own list %s", r.body)
	}
	r = c.do(http.MethodGet, "/courses?instructor_id="+instructorID, studentAT, nil)
	expect(t, r, http.StatusOK, "student lists the instructor's courses")
	if r.json(t)["total"] != float64(0) {
		t.Errorf("student sees drafts: %s", r.body)
	}

	// Refresh rotation and reuse detection.
	refresh := func(token string) response {
		return c.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refresh_token": token})
	}
	r = refresh(instructorRT)
	expect(t, r, http.StatusOK, "refresh")
	rotatedRT := r.str(t, "refresh_token")
	if rotatedRT == instructorRT {
		t.Error("refresh token was not rotated")
	}
	expect(t, c.do(http.MethodGet, "/users/me", r.str(t, "access_token"), nil), http.StatusOK, "new access token")
	expectProblem(t, refresh(instructorRT), http.StatusUnauthorized, "reuse of the rotated token")
	expectProblem(t, refresh(rotatedRT), http.StatusUnauthorized, "current token after reuse (session revoked)")
	expectProblem(t, refresh("not-a-real-token"), http.StatusUnauthorized, "unknown refresh token")
	expectProblem(t, c.do(http.MethodPost, "/auth/refresh", "", map[string]any{}), http.StatusUnprocessableEntity, "missing refresh token")

	t.Run("concurrent refreshes of one token: exactly one wins", func(t *testing.T) {
		token := login(studentEmail, "studentpass1").str(t, "refresh_token")
		var wg sync.WaitGroup
		var ok atomic.Int32
		for range 5 {
			wg.Go(func() {
				if refresh(token).status == http.StatusOK {
					ok.Add(1)
				}
			})
		}
		wg.Wait()
		if ok.Load() != 1 {
			t.Fatalf("%d refreshes succeeded, want 1", ok.Load())
		}
	})

	// Logout.
	sessionRT := login(studentEmail, "studentpass1").str(t, "refresh_token")
	logout := func(token string) response {
		return c.do(http.MethodPost, "/auth/logout", "", map[string]any{"refresh_token": token})
	}
	expect(t, logout(sessionRT), http.StatusNoContent, "logout")
	expectProblem(t, refresh(sessionRT), http.StatusUnauthorized, "refresh after logout")
	expect(t, logout(sessionRT), http.StatusNoContent, "logout again")
	expect(t, logout("unknown"), http.StatusNoContent, "logout unknown token")

	t.Run("admin", func(t *testing.T) {
		adminEmail := fmt.Sprintf("admin%d@example.com", run)
		createAdmin(t, adminEmail, "adminpassword1")
		createAdmin(t, adminEmail, "adminpassword1") // idempotent

		adminAT := login(adminEmail, "adminpassword1").str(t, "access_token")
		r := c.do(http.MethodGet, "/users/"+studentID, adminAT, nil)
		expect(t, r, http.StatusOK, "admin reads a profile")
		if r.str(t, "email") != studentEmail {
			t.Errorf("admin doesn't see the email: %s", r.body)
		}
		r = c.do(http.MethodGet, "/courses?instructor_id="+instructorID, adminAT, nil)
		expect(t, r, http.StatusOK, "admin lists")
		if r.json(t)["total"] != float64(1) {
			t.Errorf("admin doesn't see the draft: %s", r.body)
		}
		r = c.do(http.MethodPatch, "/courses/"+courseID, adminAT, map[string]any{"title": "Go 102"})
		expect(t, r, http.StatusOK, "admin edits any course")
	})

	if c.validated.Load() == 0 {
		t.Fatal("no response was validated against the spec")
	}
	t.Logf("%d responses validated against api/openapi.yaml", c.validated.Load())
}

// createAdmin runs the operator CLI against the stack's database (reads .env from the repo root).
func createAdmin(t *testing.T, email, password string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/identity-admin", "-email", email, "-name", "E2E Admin")
	cmd.Dir = testutil.RepoRoot(t)
	cmd.Stdin = strings.NewReader(password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("identity-admin: %v\n%s", err, out)
	}
}
