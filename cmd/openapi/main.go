// Command openapi writes the public API contract from the services' Huma operations:
//
//	go run ./cmd/openapi -o api/openapi.yaml
//
// Every service registers onto one API, so a path or schema-name clash between services panics here.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/api"
	"github.com/all2pie/go-smart-course/internal/catalog"
	"github.com/all2pie/go-smart-course/internal/identity"
	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
)

const description = `Public API of the SmartCourse learning platform, served by the gateway.

All requests go through the gateway (the only published port). Except for the
` + "`auth`" + ` endpoints and ` + "`/ping`" + `, every route requires ` + "`Authorization: Bearer <access_token>`" + `.

Access tokens are short-lived JWTs (15 min). Use ` + "`POST /auth/refresh`" + ` with the
refresh token to get a new pair; each refresh token works once (rotation), and
reusing an old one revokes that login session.

Every response carries an ` + "`X-Request-ID`" + ` header. Clients may send their own
(it must be a UUID, otherwise a new one is generated).

Errors are RFC 9457 problem details (` + "`application/problem+json`" + `). Malformed JSON is
a 400; a well-formed request that fails validation is a 422 with the failing fields in ` + "`errors`" + `.
`

func main() {
	out := flag.String("o", "api/openapi.yaml", "output file")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)
	a := httpapi.New(gin.New(), "SmartCourse API", api.Version)

	// Registration only reflects on types, so handlers need no live dependencies.
	identity.NewHandler(nil, nil).Register(a)
	catalog.NewHandler(nil).Register(a)
	registerPing(a)

	doc := a.OpenAPI()
	describeGateway(doc)

	b, err := doc.YAML()
	if err != nil {
		log.Fatalf("encoding spec: %v", err)
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		log.Fatalf("writing spec: %v", err)
	}
}

type pingOutput struct {
	Body struct {
		Message string `json:"message" enum:"pong"`
	}
}

// The gateway serves /ping itself (web.NewRouter); this only documents it.
func registerPing(a huma.API) {
	huma.Register(a, httpapi.Public(huma.Operation{
		OperationID: "ping",
		Method:      http.MethodGet,
		Path:        "/ping",
		Tags:        []string{"system"},
		Summary:     "Gateway liveness",
	}), func(context.Context, *struct{}) (*pingOutput, error) { return nil, nil })
}

// describeGateway adds what only the gateway knows: servers, tags, its own 401 and 502 answers.
func describeGateway(doc *huma.OpenAPI) {
	doc.Info.Description = description
	doc.Info.License = &huma.License{Name: "Proprietary", Identifier: "LicenseRef-Proprietary"}
	doc.Servers = []*huma.Server{{URL: "http://localhost:8000", Description: "Local (task up)"}}
	doc.Tags = []*huma.Tag{
		{Name: "auth", Description: "Registration and sessions (public)."},
		{Name: "users", Description: "User profiles (Identity service)."},
		{Name: "courses", Description: "Course catalog (Catalog service)."},
		{Name: "system", Description: "Liveness."},
	}

	for _, item := range doc.Paths {
		for _, op := range []*huma.Operation{item.Get, item.Post, item.Put, item.Patch, item.Delete} {
			if op == nil || op.OperationID == "ping" {
				continue
			}
			op.Responses["502"] = errorResponse(op, "The owning service is unreachable.")
			if r, ok := op.Responses["401"]; ok && len(op.Security) > 0 {
				r.Headers = map[string]*huma.Param{"WWW-Authenticate": {
					Description: "Set by the gateway when the access token is missing or invalid.",
					Schema:      &huma.Schema{Type: huma.TypeString, Examples: []any{`Bearer realm="smartcourse"`}},
				}}
			}
		}
	}
}

// errorResponse reuses the problem schema Huma already attached to the operation's other errors.
func errorResponse(op *huma.Operation, description string) *huma.Response {
	return &huma.Response{Description: description, Content: op.Responses["500"].Content}
}
