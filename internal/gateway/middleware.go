package gateway

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/problem"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

// RequestID keeps a client-sent X-Request-ID only if it is a UUID, so arbitrary strings never reach logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.GetHeader(web.HeaderRequestID))
		if err != nil {
			id = uuid.New()
		}
		c.Request.Header.Set(web.HeaderRequestID, id.String())
		c.Header(web.HeaderRequestID, id.String())
		c.Next()
	}
}

// StripIdentityHeaders drops client-sent caller headers; only Authenticate may set them.
func StripIdentityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(auth.HeaderUserID)
		c.Request.Header.Del(auth.HeaderUserRole)
		c.Next()
	}
}

// Authenticate verifies the bearer access token and forwards the caller as X-User-* headers.
// The token itself is not forwarded: services trust the gateway, not the client.
func Authenticate(tokens *auth.Tokens) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") {
			unauthorized(c)
			return
		}

		actor, err := tokens.Verify(token)
		if err != nil {
			unauthorized(c)
			return
		}

		c.Request.Header.Del("Authorization")
		c.Request.Header.Set(auth.HeaderUserID, actor.ID.String())
		c.Request.Header.Set(auth.HeaderUserRole, string(actor.Role))
		c.Next()
	}
}

func unauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer realm="smartcourse"`)
	problem.Write(c.Writer, http.StatusUnauthorized, "authentication required")
	c.Abort()
}
