// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package middleware

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a CORS middleware configured from environment variables
func CORS() gin.HandlerFunc {
	validateCORSConfig()

	corsOrigin := getEnvOrDefault("CORS_ORIGIN", "http://localhost:3000")
	allowedOrigins := parseOrigins(corsOrigin)

	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return sharedmiddleware.OriginAllowed(allowedOrigins, origin)
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	return cors.New(config)
}

// corsValidationOnce ensures the wildcard + credentials check runs only once.
var corsValidationOnce sync.Once

// validateCORSConfig fails-fast at startup if CORS_ORIGIN=* is set while
// AllowCredentials is enabled. A wildcard origin with credentials allows any
// website to make authenticated cross-origin requests (H9 + L12).
func validateCORSConfig() {
	corsValidationOnce.Do(func() {
		corsOrigin := strings.TrimSpace(getEnvOrDefault("CORS_ORIGIN", "http://localhost:3000"))
		for _, o := range strings.Split(corsOrigin, ",") {
			if strings.TrimSpace(o) == "*" {
				log.Fatal("CORS_ORIGIN=* is not allowed when AllowCredentials is enabled; " +
					"configure explicit origins via the CORS_ORIGIN env var")
			}
		}
	})
}

// LoadHTTPAllowedOrigins reads CORS_ORIGIN (comma-separated) and returns the
// HTTP allowlist. Shared by the CORS middleware and the CSRF OriginCheck
// (H3). Default is http://localhost:3000 for local dev.
func LoadHTTPAllowedOrigins() []string {
	corsOrigin := getEnvOrDefault("CORS_ORIGIN", "http://localhost:3000")
	return parseOrigins(corsOrigin)
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
