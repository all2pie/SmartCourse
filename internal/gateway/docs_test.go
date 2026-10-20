package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/api"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

func newDocsRouter(t *testing.T, withUI bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, err := web.NewRouter("test")
	if err != nil {
		t.Fatal(err)
	}
	RegisterDocs(router, withUI)
	return router
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestDocsUIIsSelfContained(t *testing.T) {
	router := newDocsRouter(t, true)

	page := get(router, "/docs")
	if page.Code != http.StatusOK {
		t.Fatalf("/docs = %d", page.Code)
	}
	if strings.Contains(page.Body.String(), "//") {
		t.Fatalf("docs page references another origin:\n%s", page.Body)
	}

	refs := regexp.MustCompile(`(?:src|href)="(/[^"]+)"`).FindAllStringSubmatch(page.Body.String(), -1)
	if len(refs) != 3 {
		t.Fatalf("found %d asset references, want 3 (css, bundle, init)", len(refs))
	}
	paths := []string{"/docs", "/docs/assets/swagger-ui-bundle.js.LICENSE.txt"}
	for _, ref := range refs {
		paths = append(paths, ref[1])
	}
	for _, path := range paths {
		rec := get(router, path)
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Errorf("%s = %d, %d bytes", path, rec.Code, rec.Body.Len())
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.HasPrefix(csp, "default-src 'self';") {
			t.Errorf("%s: Content-Security-Policy %q", path, csp)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", path)
		}
	}

	if init := get(router, "/docs/assets/init.js").Body.String(); !strings.Contains(init, "validatorUrl: null") {
		t.Errorf("init.js lets Swagger UI call the external validator: %s", init)
	}
	if spec := get(router, "/openapi.yaml"); !bytes.Equal(spec.Body.Bytes(), api.OpenAPI) {
		t.Error("/openapi.yaml is not the embedded spec")
	}
}

func TestDocsWithoutUIServesOnlyTheSpec(t *testing.T) {
	router := newDocsRouter(t, false)

	if rec := get(router, "/openapi.yaml"); rec.Code != http.StatusOK {
		t.Fatalf("/openapi.yaml = %d", rec.Code)
	}
	for _, path := range []string{"/docs", "/docs/assets/swagger-ui-bundle.js"} {
		testutil.Problem(t, get(router, path), http.StatusNotFound)
	}
}
