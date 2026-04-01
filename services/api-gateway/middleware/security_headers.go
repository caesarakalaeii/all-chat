package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders adds browser-side security headers recommended by OWASP.
// A Content-Security-Policy is intentionally omitted for now because the
// overlay pages load external fonts, emote CDNs, and user-supplied CSS.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}
