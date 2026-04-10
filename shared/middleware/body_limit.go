package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit returns middleware that rejects request bodies exceeding maxBytes.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
