package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
)

// RegisterRoutes forwards paths, unchanged, to the owning service. Only the routes listed
// as public skip authentication; anything added to the protected group requires a token.
func RegisterRoutes(router gin.IRouter, cfg Config, tokens *auth.Tokens) {
	identity := newProxy(cfg.IdentityURL)
	catalog := newProxy(cfg.CatalogURL)

	for _, path := range []string{"/auth/register", "/auth/login", "/auth/refresh", "/auth/logout"} {
		router.POST(path, identity)
	}

	protected := router.Group("", Authenticate(tokens))
	protected.Any("/users", identity)
	protected.Any("/users/*path", identity)
	protected.Any("/courses", catalog)
	protected.Any("/courses/*path", catalog)
}

func newProxy(target url.URL) gin.HandlerFunc {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(&target)
			r.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy %s %q to %s: %v", r.Method, r.URL.Path, target.Host, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
		},
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
