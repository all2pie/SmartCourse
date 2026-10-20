// Package httpapi is the Huma setup shared by the service APIs: OpenAPI settings, the forwarded
// caller, and conversion of domain errors into problem details.
package httpapi

import (
	"context"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
	"github.com/all2pie/go-smart-course/internal/platform/problem"
)

// SecurityScheme is the OpenAPI name of the bearer access token.
const SecurityScheme = "bearerAuth"

var errUnauthenticated = errs.New(errs.Unauthenticated, "authentication required")

// New mounts a Huma API on router.
func New(router *gin.Engine, title, version string) huma.API {
	return Init(humagin.New(router, Config(title, version)))
}

// Config returns the Huma settings every service API shares.
func Config(title, version string) huma.Config {
	// Lists are always returned as arrays, never null.
	huma.DefaultArrayNullable = false

	cfg := huma.DefaultConfig(title, version)
	// The gateway serves the single public document, so services expose no docs routes.
	cfg.OpenAPIPath, cfg.DocsPath, cfg.SchemasPath = "", "", ""
	// Drops the default hook that adds `$schema` and Link headers to responses.
	cfg.CreateHooks = nil
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		SecurityScheme: {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "Access token from `POST /auth/login` or `POST /auth/refresh`.",
		},
	}
	return cfg
}

// Init adds the shared middleware; New calls it, tests call it on humatest APIs.
func Init(api huma.API) huma.API {
	api.UseMiddleware(withActor)
	return api
}

// Public marks an operation that needs no access token.
func Public(op huma.Operation) huma.Operation {
	op.Security = []map[string][]string{}
	return op
}

// Protected marks an operation that needs an access token.
func Protected(op huma.Operation) huma.Operation {
	op.Security = []map[string][]string{{SecurityScheme: {}}}
	op.Errors = append(op.Errors, http.StatusUnauthorized)
	return op
}

// Register adds an operation whose handler returns domain errors, converting them to problems.
func Register[I, O any](api huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	// Huma documents 422 for invalid bodies but not 400 for malformed JSON.
	if _, ok := reflect.TypeFor[I]().FieldByName("Body"); ok {
		op.Errors = append(op.Errors, http.StatusBadRequest)
	}
	huma.Register(api, op, func(ctx context.Context, in *I) (*O, error) {
		out, err := handler(ctx, in)
		if err != nil {
			return nil, problem.From(err)
		}
		return out, nil
	})
}

type actorKey struct{}

// Actor returns the caller identified by the gateway-forwarded headers.
func Actor(ctx context.Context) (auth.Actor, error) {
	a, ok := ctx.Value(actorKey{}).(auth.Actor)
	if !ok {
		return auth.Actor{}, errUnauthenticated
	}
	return a, nil
}

func withActor(ctx huma.Context, next func(huma.Context)) {
	id, err := uuid.Parse(ctx.Header(auth.HeaderUserID))
	role := auth.Role(ctx.Header(auth.HeaderUserRole))
	if err == nil && role.Valid() {
		ctx = huma.WithValue(ctx, actorKey{}, auth.Actor{ID: id, Role: role})
	}
	next(ctx)
}
