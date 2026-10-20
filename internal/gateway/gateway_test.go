package gateway

import (
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

// upstream records the last request it received.
type upstream struct {
	*httptest.Server
	last atomic.Pointer[http.Request]
}

func newUpstream(t *testing.T) *upstream {
	u := &upstream{}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.last.Store(r)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(u.Close)
	return u
}

func newGateway(t *testing.T, identity, catalog string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, err := web.NewRouter("test")
	if err != nil {
		t.Fatal(err)
	}
	router.Use(RequestID(), StripIdentityHeaders())

	parse := func(s string) url.URL {
		u, err := url.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		return *u
	}
	RegisterRoutes(router, Config{IdentityURL: parse(identity), CatalogURL: parse(catalog)}, testutil.Tokens(t))

	// A real listener: Gin's CloseNotify panics on a ResponseRecorder inside ReverseProxy.
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv.URL
}

func do(t *testing.T, gw, method, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, gw+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	rec := httptest.NewRecorder()
	maps.Copy(rec.Header(), res.Header)
	rec.WriteHeader(res.StatusCode)
	if _, err := io.Copy(rec, res.Body); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestRequestID(t *testing.T) {
	up := newUpstream(t)
	gw := newGateway(t, up.URL, up.URL)

	sent := uuid.NewString()
	rec := do(t, gw, http.MethodPost, "/auth/login", http.Header{web.HeaderRequestID: {sent}})
	if got := rec.Header().Get(web.HeaderRequestID); got != sent {
		t.Fatalf("echoed %q, want the client's UUID %q", got, sent)
	}
	if got := up.last.Load().Header.Get(web.HeaderRequestID); got != sent {
		t.Fatalf("forwarded %q, want %q", got, sent)
	}

	rec = do(t, gw, http.MethodPost, "/auth/login", http.Header{web.HeaderRequestID: {"<script>"}})
	got := rec.Header().Get(web.HeaderRequestID)
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("non-UUID request ID was not replaced: %q", got)
	}
}

func TestPublicRoutesStripSpoofedIdentity(t *testing.T) {
	up := newUpstream(t)
	gw := newGateway(t, up.URL, up.URL)

	rec := do(t, gw, http.MethodPost, "/auth/register", http.Header{
		auth.HeaderUserID:   {uuid.NewString()},
		auth.HeaderUserRole: {"admin"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if h := up.last.Load().Header; h.Get(auth.HeaderUserID) != "" || h.Get(auth.HeaderUserRole) != "" {
		t.Fatalf("spoofed identity headers reached the service: %v", h)
	}
}

func TestAuthenticate(t *testing.T) {
	up := newUpstream(t)
	gw := newGateway(t, up.URL, up.URL)
	tokens := testutil.Tokens(t)
	actor := testutil.Actor(auth.RoleInstructor)

	valid, err := tokens.Sign(actor, time.Minute, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	expired, err := tokens.Sign(actor, time.Minute, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	for name, authz := range map[string]string{
		"missing":   "",
		"garbage":   "Bearer garbage",
		"expired":   "Bearer " + expired,
		"tampered":  "Bearer " + valid[:len(valid)-2] + "xx",
		"basic":     "Basic dXNlcjpwYXNz",
		"no scheme": valid,
	} {
		t.Run(name, func(t *testing.T) {
			up.last.Store(nil)
			rec := do(t, gw, http.MethodGet, "/courses", http.Header{"Authorization": {authz}})
			testutil.Problem(t, rec, http.StatusUnauthorized)
			if rec.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing WWW-Authenticate")
			}
			if up.last.Load() != nil {
				t.Fatal("unauthenticated request reached the service")
			}
		})
	}

	t.Run("valid token forwards the caller, not the token", func(t *testing.T) {
		rec := do(t, gw, http.MethodGet, "/users/me", http.Header{
			"Authorization":     {"bearer " + valid},
			auth.HeaderUserRole: {"admin"},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		h := up.last.Load().Header
		if h.Get(auth.HeaderUserID) != actor.ID.String() || h.Get(auth.HeaderUserRole) != string(actor.Role) {
			t.Fatalf("forwarded caller = %q/%q", h.Get(auth.HeaderUserID), h.Get(auth.HeaderUserRole))
		}
		if h.Get("Authorization") != "" {
			t.Fatal("Authorization header was forwarded")
		}
	})
}

func TestRoutingErrors(t *testing.T) {
	up := newUpstream(t)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	gw := newGateway(t, up.URL, down.URL)

	token, err := testutil.Tokens(t).Sign(testutil.Actor(auth.RoleStudent), time.Minute, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	bearer := http.Header{"Authorization": {"Bearer " + token}}

	testutil.Problem(t, do(t, gw, http.MethodGet, "/courses", bearer), http.StatusBadGateway)
	testutil.Problem(t, do(t, gw, http.MethodGet, "/nope", nil), http.StatusNotFound)
	testutil.Problem(t, do(t, gw, http.MethodGet, "/auth/login", nil), http.StatusMethodNotAllowed)
}
