package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes forwards each public path prefix, unchanged, to the service that owns it.
func RegisterRoutes(router gin.IRouter, cfg Config) {
	identity := newProxy(cfg.IdentityURL)
	catalog := newProxy(cfg.CatalogURL)

	router.Any("/users", identity)
	router.Any("/users/*path", identity)
	router.Any("/courses", catalog)
	router.Any("/courses/*path", catalog)
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
