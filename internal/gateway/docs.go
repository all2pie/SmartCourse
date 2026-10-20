package gateway

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/api"
)

// Swagger UI is vendored by `task docs:update` (version in swaggerui/VERSION) so /docs needs no CDN.
var (
	//go:embed swaggerui/swagger-ui-bundle.js
	swaggerBundle []byte
	//go:embed swaggerui/swagger-ui-bundle.js.LICENSE.txt
	swaggerLicenses []byte
	//go:embed swaggerui/swagger-ui.css
	swaggerCSS []byte
)

const docsPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>SmartCourse API</title>
  <link rel="icon" href="data:,">
  <link rel="stylesheet" href="/docs/assets/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/docs/assets/swagger-ui-bundle.js"></script>
  <script src="/docs/assets/init.js"></script>
</body>
</html>`

// validatorUrl null stops Swagger UI loading a badge from validator.swagger.io.
const docsInit = `window.ui = SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui", persistAuthorization: true, validatorUrl: null });`

// Everything is same-origin; Swagger UI needs inline style attributes and data: images.
const docsCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

const contentTypeJS = "text/javascript; charset=utf-8"

// RegisterDocs serves the OpenAPI contract and, if withUI, a self-contained Swagger UI at /docs. Both are public.
func RegisterDocs(router gin.IRouter, withUI bool) {
	router.GET("/openapi.yaml", serve("application/yaml", api.OpenAPI))
	if !withUI {
		return
	}

	router.GET("/docs", serve("text/html; charset=utf-8", []byte(docsPage)))
	router.GET("/docs/assets/init.js", serve(contentTypeJS, []byte(docsInit)))
	router.GET("/docs/assets/swagger-ui-bundle.js", serve(contentTypeJS, swaggerBundle))
	router.GET("/docs/assets/swagger-ui-bundle.js.LICENSE.txt", serve("text/plain; charset=utf-8", swaggerLicenses))
	router.GET("/docs/assets/swagger-ui.css", serve("text/css; charset=utf-8", swaggerCSS))
}

func serve(contentType string, body []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", docsCSP)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, contentType, body)
	}
}
