// Package api holds the public OpenAPI contract, embedded so the gateway can serve it.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
