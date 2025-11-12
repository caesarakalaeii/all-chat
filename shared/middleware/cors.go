package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns a Gin middleware for handling CORS
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	// Default origins if none provided
	origins := []string{"http://localhost:3000", "http://localhost:8080"}

	if len(allowedOrigins) > 0 {
		origins = append(origins, allowedOrigins...)
	}

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

// CORS is a convenience function for default CORS settings
func CORS() gin.HandlerFunc {
	return CORSMiddleware(nil)
}
