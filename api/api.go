// Package api holds the public OpenAPI contract, embedded so the gateway can serve it.
// The file is generated from the services' operations by cmd/openapi (`task api:gen`).
package api

import _ "embed"

// Version is the public API version, shared by the services' and the gateway's documents.
const Version = "0.2.0"

//go:embed openapi.yaml
var OpenAPI []byte
