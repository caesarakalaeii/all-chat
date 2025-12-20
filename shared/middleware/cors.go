package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins []string
	Environment    string
	Logger         *zap.Logger
}

// CORSMiddleware returns a Gin middleware for handling CORS with environment-aware defaults
func CORSMiddleware(cfg CORSConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	origins := buildAllowedOrigins(cfg)
	isDev := strings.ToLower(cfg.Environment) == "development" || cfg.Environment == "dev" || cfg.Environment == ""

	cfg.Logger.Info("CORS configured",
		zap.Strings("allowed_origins", origins),
		zap.String("environment", cfg.Environment),
		zap.Bool("allow_extension_wildcard", isDev))

	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	// In development, allow chrome-extension://* via custom origin function
	if isDev {
		corsConfig.AllowOriginFunc = func(origin string) bool {
			// Check if origin is in the allowed list
			for _, allowed := range origins {
				if origin == allowed {
					return true
				}
			}
			// In dev, allow any chrome-extension:// origin
			return strings.HasPrefix(origin, "chrome-extension://")
		}
	} else {
		// In production, use explicit origin list only
		corsConfig.AllowOrigins = origins
	}

	return cors.New(corsConfig)
}

// buildAllowedOrigins determines the list of allowed origins based on environment
func buildAllowedOrigins(cfg CORSConfig) []string {
	var origins []string

	// Environment-specific defaults
	switch strings.ToLower(cfg.Environment) {
	case "production", "prod":
		// Production origins
		origins = []string{
			"https://allch.at",
			"https://www.allch.at",
		}

	case "staging", "stage":
		// Staging origins
		origins = []string{
			"https://staging.allch.at",
			"http://localhost:3000", // For local testing against staging
		}

	case "development", "dev", "":
		// Development origins
		origins = []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
		}

	default:
		// Unknown environment - use development defaults
		origins = []string{
			"http://localhost:3000",
			"http://localhost:8080",
		}
	}

	// Add custom origins from config
	if len(cfg.AllowedOrigins) > 0 {
		origins = append(origins, cfg.AllowedOrigins...)
	}

	// Add browser extension origins if specified
	if extensionID := os.Getenv("BROWSER_EXTENSION_ID"); extensionID != "" {
		origins = append(origins, "chrome-extension://"+extensionID)
	}

	// Support wildcard chrome-extension in development
	if cfg.Environment == "development" || cfg.Environment == "dev" || cfg.Environment == "" {
		// In development, we can't use wildcard, but we can allow a pattern
		// Note: gin-contrib/cors doesn't support wildcards, so we need to allow specific extension IDs
		// For now, we'll document that extension ID must be provided
	}

	return origins
}

// CORS is a convenience function for default CORS settings
// Deprecated: Use CORSMiddleware with explicit config instead
func CORS() gin.HandlerFunc {
	return CORSMiddleware(CORSConfig{
		Environment: os.Getenv("ENVIRONMENT"),
	})
}

// CORSFromEnv creates CORS middleware from environment variables
// Reads: ENVIRONMENT, CORS_ORIGINS (comma-separated), BROWSER_EXTENSION_ID
func CORSFromEnv(logger *zap.Logger) gin.HandlerFunc {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	var customOrigins []string
	if corsOrigins := os.Getenv("CORS_ORIGINS"); corsOrigins != "" {
		customOrigins = strings.Split(corsOrigins, ",")
		for i, origin := range customOrigins {
			customOrigins[i] = strings.TrimSpace(origin)
		}
	}

	return CORSMiddleware(CORSConfig{
		AllowedOrigins: customOrigins,
		Environment:    env,
		Logger:         logger,
	})
}
