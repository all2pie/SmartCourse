package gateway

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
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

// StripIdentityHeaders drops client-sent caller headers; only the gateway may set them (after JWT checks in 1.6).
func StripIdentityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(auth.HeaderUserID)
		c.Request.Header.Del(auth.HeaderUserRole)
		c.Next()
	}
}
