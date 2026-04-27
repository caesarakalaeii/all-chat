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
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/auth"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Build a test KeyChain with a legacy secret so legacy tokens are accepted too
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-key-v1")},
		[]byte("test-secret-key"),
		"v1",
	)

	// Generate a valid legacy token (no kid) for backward-compat testing
	validToken, err := auth.GenerateJWT("user-123", "twitch-123", "testuser", "test-secret-key", false)
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
		checkContext   func(*testing.T, *gin.Context)
	}{
		{
			name:           "valid legacy token",
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
			router := gin.New()
			router.Use(JWTAuth(kc))

			var capturedContext *gin.Context
			router.GET("/test", func(c *gin.Context) {
				capturedContext = c
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

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

// TestInternalServiceAuth is the api-gateway parallel-bug regression test.
// It proves that the /internal route group only accepts tokens signed with the
// SERVICE_JWT_SECRET chain, NOT the JWT_SECRET (user) chain (fixes line 564 bug).
func TestInternalServiceAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userKC := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("user-chain-secret")},
		nil,
		"v1",
	)
	serviceKC := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("service-chain-secret")},
		nil,
		"v1",
	)

	// Token issued by the service chain
	serviceToken, err := auth.GenerateServiceJWTWithKid(
		serviceKC.LatestKid(),
		"share-service",
		string(serviceKC.LatestSecret()),
		30*time.Second,
	)
	require.NoError(t, err)

	// Token issued by the user chain (wrong chain for internal route)
	userToken, err := auth.GenerateServiceJWTWithKid(
		userKC.LatestKid(),
		"share-service",
		string(userKC.LatestSecret()),
		30*time.Second,
	)
	require.NoError(t, err)

	// The internal route group uses ServiceJWTAuth with serviceKeyChain (the fix)
	router := gin.New()
	internal := router.Group("/internal")
	internal.Use(sharedmiddleware.ServiceJWTAuth(serviceKC, "share-service", "overlay-manager", "auth-service"))
	internal.POST("/ws/notify", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Service-chain token must be accepted
	req := httptest.NewRequest(http.MethodPost, "/internal/ws/notify", nil)
	req.Header.Set("Authorization", "Bearer "+serviceToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "service-chain token must be accepted on /internal route")

	// User-chain token must be REJECTED (the bug was that this was accepted)
	req = httptest.NewRequest(http.MethodPost, "/internal/ws/notify", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "user-chain token must NOT be accepted on /internal route")
}
