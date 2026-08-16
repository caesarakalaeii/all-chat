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

package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestProxyHandler_ForwardRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestPath    string
		requestMethod  string
		requestBody    string
		requestHeaders map[string]string
		backendHandler http.HandlerFunc
		expectedStatus int
		expectedBody   string
		checkHeaders   func(*testing.T, http.Header)
	}{
		{
			name:          "GET request forwarded successfully",
			requestPath:   "/api/v1/emotes/channel/xqc",
			requestMethod: http.MethodGet,
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/emotes/channel/xqc", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"channel":"xqc","emotes":[]}`))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"channel":"xqc","emotes":[]}`,
		},
		{
			name:          "POST request with body forwarded",
			requestPath:   "/api/v1/overlays",
			requestMethod: http.MethodPost,
			requestBody:   `{"name":"My Overlay"}`,
			requestHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/overlays", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				body, _ := io.ReadAll(r.Body)
				assert.JSONEq(t, `{"name":"My Overlay"}`, string(body))

				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"id":"123","name":"My Overlay"}`))
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"id":"123","name":"My Overlay"}`,
		},
		{
			name:          "headers are forwarded",
			requestPath:   "/api/v1/auth/me",
			requestMethod: http.MethodGet,
			requestHeaders: map[string]string{
				"Authorization": "Bearer test-token",
				"X-Custom":      "test-value",
			},
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "test-value", r.Header.Get("X-Custom"))

				w.Header().Set("X-Response-Header", "response-value")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id":"user-123"}`))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"id":"user-123"}`,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "response-value", headers.Get("X-Response-Header"))
			},
		},
		{
			// The reason every backend must wire its own PAT resolver: the gateway does
			// not translate a personal access token into anything else, it forwards the
			// client's Authorization header byte-for-byte and each service re-validates
			// independently. A service without middleware.SetAPITokenResolver would see
			// `allchat_pat_…` and reject it as a malformed JWT.
			name:          "personal access token bearer is forwarded verbatim",
			requestPath:   "/api/v1/auth/me/api-tokens",
			requestMethod: http.MethodGet,
			requestHeaders: map[string]string{
				"Authorization": "Bearer allchat_pat_forwarded-unchanged",
			},
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer allchat_pat_forwarded-unchanged", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"tokens":[]}`))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"tokens":[]}`,
		},
		{
			name:          "query parameters are preserved",
			requestPath:   "/api/v1/overlays?limit=10&offset=20",
			requestMethod: http.MethodGet,
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "10", r.URL.Query().Get("limit"))
				assert.Equal(t, "20", r.URL.Query().Get("offset"))

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"overlays":[]}`))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"overlays":[]}`,
		},
		{
			name:          "backend error is forwarded",
			requestPath:   "/api/v1/overlays/999",
			requestMethod: http.MethodGet,
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":"overlay not found"}`))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"overlay not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock backend server
			backend := httptest.NewServer(tt.backendHandler)
			defer backend.Close()

			// Create service registry
			registry := &models.ServiceRegistry{
				Services: map[string]*models.ServiceConfig{
					"auth-service": {
						Name:       "auth-service",
						BaseURL:    backend.URL,
						PathPrefix: "/api/v1/auth",
					},
					"overlay-manager": {
						Name:       "overlay-manager",
						BaseURL:    backend.URL,
						PathPrefix: "/api/v1/overlays",
					},
					"emote-service": {
						Name:          "emote-service",
						BaseURL:       backend.URL,
						PathPrefix:    "/api/v1/emotes",
						StripPrefix:   true,
						RewritePrefix: "/emotes",
					},
				},
			}

			// Create proxy handler
			handler := NewProxyHandler(registry)

			// Create test router
			router := gin.New()
			router.Any("/api/v1/*path", handler.ForwardRequest)

			// Create request
			var body io.Reader
			if tt.requestBody != "" {
				body = strings.NewReader(tt.requestBody)
			}

			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, body)
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				// Try JSON comparison first, fall back to string comparison
				if strings.HasPrefix(tt.expectedBody, "{") || strings.HasPrefix(tt.expectedBody, "[") {
					assert.JSONEq(t, tt.expectedBody, w.Body.String())
				} else {
					assert.Equal(t, tt.expectedBody, w.Body.String())
				}
			}

			if tt.checkHeaders != nil {
				tt.checkHeaders(t, w.Header())
			}
		})
	}
}

func TestProxyHandler_NoMatchingService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &models.ServiceRegistry{
		Services: map[string]*models.ServiceConfig{
			"auth-service": {
				Name:       "auth-service",
				BaseURL:    "http://localhost:8081",
				PathPrefix: "/api/v1/auth",
			},
		},
	}

	handler := NewProxyHandler(registry)
	router := gin.New()
	router.Any("/api/v1/*path", handler.ForwardRequest)

	// Request to unknown path
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "no service found")
}

func TestProxyHandler_BackendUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create registry with unavailable backend
	registry := &models.ServiceRegistry{
		Services: map[string]*models.ServiceConfig{
			"auth-service": {
				Name:       "auth-service",
				BaseURL:    "http://localhost:9999", // Non-existent service
				PathPrefix: "/api/v1/auth",
			},
		},
	}

	handler := NewProxyHandler(registry)
	router := gin.New()
	router.Any("/api/v1/*path", handler.ForwardRequest)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/twitch/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "backend service unavailable")
}
