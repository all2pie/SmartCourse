package gateway

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/api"
)

const swaggerUIVersion = "5.34.0"

var docsPage = []byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>SmartCourse API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui", persistAuthorization: true });
    };
  </script>
</body>
</html>`)

// RegisterDocs serves the OpenAPI contract and a Swagger UI page; both are public.
func RegisterDocs(router gin.IRouter) {
	router.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml", api.OpenAPI)
	})
	router.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", docsPage)
	})
}
