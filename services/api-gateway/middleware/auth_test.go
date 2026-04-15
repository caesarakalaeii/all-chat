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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTAuth(t *testing.T) {
	// JWT secret for testing
	jwtSecret := "test-secret-key"
	os.Setenv("JWT_SECRET", jwtSecret)
	defer os.Unsetenv("JWT_SECRET")

	// Generate a valid token for testing
	validToken, err := auth.GenerateJWT("user-123", "twitch-123", "testuser", jwtSecret, false)
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
		checkContext   func(*testing.T, *gin.Context)
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, c *gin.Context) {
				userID, exists := c.Get("user_id")
				assert.True(t, exists)
				assert.Equal(t, "user-123", userID)

				username, exists := c.Get("username")
				assert.True(t, exists)
				assert.Equal(t, "testuser", username)
			},
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"missing authorization header"}`,
		},
		{
			name:           "invalid authorization format - no Bearer prefix",
			authHeader:     validToken,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"invalid authorization format, expected 'Bearer <token>'"}`,
		},
		{
			name:           "empty token after Bearer",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"empty token"}`,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"invalid or expired token"}`,
		},
		{
			name:           "malformed token",
			authHeader:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"invalid or expired token"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set Gin to test mode
			gin.SetMode(gin.TestMode)

			// Create router with middleware
			router := gin.New()
			router.Use(JWTAuth(jwtSecret))

			// Add test endpoint
			var capturedContext *gin.Context
			router.GET("/test", func(c *gin.Context) {
				capturedContext = c
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Perform request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, w.Body.String())
			}

			if tt.checkContext != nil && capturedContext != nil {
				tt.checkContext(t, capturedContext)
			}
		})
	}
}
