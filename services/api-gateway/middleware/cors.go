package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"os"
	"strings"
	"time"
)

// CORS returns a CORS middleware configured from environment variables
func CORS() gin.HandlerFunc {
	corsOrigin := getEnvOrDefault("CORS_ORIGIN", "http://localhost:3000")
	allowedOrigins := parseOrigins(corsOrigin)

	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Check exact matches first
			for _, allowed := range allowedOrigins {
				if allowed == "*" {
					return true
				}
				if allowed == origin {
					return true
				}
				// Handle wildcard patterns (e.g., chrome-extension://*, moz-extension://*)
				if strings.HasSuffix(allowed, "/*") {
					prefix := strings.TrimSuffix(allowed, "/*")
					if strings.HasPrefix(origin, prefix) {
						return true
					}
				}
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	return cors.New(config)
}

// parseOrigins parses a comma-separated list of origins
func parseOrigins(origins string) []string {
	if origins == "*" {
		return []string{"*"}
	}

	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
