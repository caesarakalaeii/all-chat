package middleware

import (
	"time"

	sharedmw "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logging returns a middleware that logs HTTP requests.
// Client IPs are anonymised (last octet zeroed) per DSGVO data-minimisation.
func Logging(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get response status
		status := c.Writer.Status()

		// Log request details
		log.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", sharedmw.AnonymizeIP(c.ClientIP())),
			zap.String("user_agent", c.Request.UserAgent()),
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				log.Error("Request error",
					zap.String("path", path),
					zap.Error(err),
				)
			}
		}
	}
}
