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

package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServiceRegistry(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		wantErr   bool
		checkFunc func(*testing.T, *ServiceRegistry)
	}{
		{
			name: "default values when env vars not set",
			envVars: map[string]string{
				// Clear env vars
				"AUTH_SERVICE_URL":    "",
				"OVERLAY_SERVICE_URL": "",
				"EMOTE_SERVICE_URL":   "",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, sr *ServiceRegistry) {
				assert.Equal(t, "http://localhost:8081", sr.Services["auth-service"].BaseURL)
				assert.Equal(t, "http://localhost:8082", sr.Services["overlay-manager"].BaseURL)
				assert.Equal(t, "http://localhost:8083", sr.Services["emote-service"].BaseURL)
			},
		},
		{
			name: "uses environment variables when set",
			envVars: map[string]string{
				"AUTH_SERVICE_URL":    "http://auth:8081",
				"OVERLAY_SERVICE_URL": "http://overlay:8082",
				"EMOTE_SERVICE_URL":   "http://emote:8083",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, sr *ServiceRegistry) {
				assert.Equal(t, "http://auth:8081", sr.Services["auth-service"].BaseURL)
				assert.Equal(t, "http://overlay:8082", sr.Services["overlay-manager"].BaseURL)
				assert.Equal(t, "http://emote:8083", sr.Services["emote-service"].BaseURL)
			},
		},
		{
			name: "validates all services are configured",
			envVars: map[string]string{
				"AUTH_SERVICE_URL":    "http://auth:8081",
				"OVERLAY_SERVICE_URL": "http://overlay:8082",
				"EMOTE_SERVICE_URL":   "http://emote:8083",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, sr *ServiceRegistry) {
				assert.Len(t, sr.Services, 22) // 4 base + 7 admin + 5 share-service + 2 maintenance routes + 1 test-stream + 1 moderation + 2 payment
				assert.NotNil(t, sr.Services["auth-service"])
				assert.NotNil(t, sr.Services["payment-service"])
				assert.NotNil(t, sr.Services["payment-webhooks"])
				assert.NotNil(t, sr.Services["overlay-manager"])
				assert.NotNil(t, sr.Services["youtube-resolver"])
				assert.NotNil(t, sr.Services["emote-service"])
				assert.NotNil(t, sr.Services["message-processor-test-stream"])
				assert.NotNil(t, sr.Services["moderation-service"])
				assert.NotNil(t, sr.Services["admin-users"])
				assert.NotNil(t, sr.Services["admin-overlays"])
				assert.NotNil(t, sr.Services["admin-user-overlays"])
				assert.NotNil(t, sr.Services["admin-sources"])
				assert.NotNil(t, sr.Services["admin-stats"])
				assert.NotNil(t, sr.Services["admin-viewers"])
				assert.NotNil(t, sr.Services["admin-cosmetics"])
				assert.NotNil(t, sr.Services["share-service-shares"])
				assert.NotNil(t, sr.Services["share-service-users"])
				assert.NotNil(t, sr.Services["share-service-admin-premium"])
				assert.NotNil(t, sr.Services["share-service-admin-feature-gates"])
				assert.NotNil(t, sr.Services["share-service-admin-beta-tester"])
				assert.NotNil(t, sr.Services["admin-maintenance"])
				assert.NotNil(t, sr.Services["maintenance-upcoming"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}
			defer func() {
				// Clean up
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			registry, err := NewServiceRegistry()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, registry)
				if tt.checkFunc != nil {
					tt.checkFunc(t, registry)
				}
			}
		})
	}
}

func TestServiceRegistry_GetServiceForPath(t *testing.T) {
	registry := &ServiceRegistry{
		Services: map[string]*ServiceConfig{
			"auth-service": {
				Name:       "auth-service",
				BaseURL:    "http://auth:8081",
				PathPrefix: "/api/v1/auth",
			},
			"overlay-manager": {
				Name:       "overlay-manager",
				BaseURL:    "http://overlay:8082",
				PathPrefix: "/api/v1/overlays",
			},
			"emote-service": {
				Name:       "emote-service",
				BaseURL:    "http://emote:8083",
				PathPrefix: "/api/v1/emotes",
			},
		},
	}

	tests := []struct {
		name            string
		path            string
		expectedService string // Service name or empty if no match
	}{
		{
			name:            "auth login path",
			path:            "/api/v1/auth/twitch/login",
			expectedService: "auth-service",
		},
		{
			name:            "auth callback path",
			path:            "/api/v1/auth/twitch/callback",
			expectedService: "auth-service",
		},
		{
			name:            "overlays list path",
			path:            "/api/v1/overlays",
			expectedService: "overlay-manager",
		},
		{
			name:            "overlays specific overlay path",
			path:            "/api/v1/overlays/123",
			expectedService: "overlay-manager",
		},
		{
			name:            "emotes channel path",
			path:            "/api/v1/emotes/channel/xqc",
			expectedService: "emote-service",
		},
		{
			name:            "emotes 7tv path",
			path:            "/api/v1/emotes/7tv/xqc",
			expectedService: "emote-service",
		},
		{
			name:            "unknown path",
			path:            "/api/v1/unknown",
			expectedService: "",
		},
		{
			name:            "root path",
			path:            "/",
			expectedService: "",
		},
		{
			name:            "health path - no match",
			path:            "/health",
			expectedService: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := registry.GetServiceForPath(tt.path)

			if tt.expectedService == "" {
				assert.Nil(t, service)
			} else {
				require.NotNil(t, service)
				assert.Equal(t, tt.expectedService, service.Name)
			}
		})
	}
}

// TestServiceRegistry_AdminUserOverlaysRouting guards against a routing
// regression where the admin "overlays for a user" endpoint was reachable only
// via /api/v1/admin/users/:id/overlays. Because the gateway matches by longest
// static prefix and the variable :id precedes the distinguishing /overlays
// suffix, that path always matched the /api/v1/admin/users prefix and was
// proxied to auth-service (which has no such route -> 404), while
// /api/v1/admin/users/:id/impersonate must still reach auth-service.
func TestServiceRegistry_AdminUserOverlaysRouting(t *testing.T) {
	for _, key := range []string{"AUTH_SERVICE_URL", "OVERLAY_SERVICE_URL", "EMOTE_SERVICE_URL"} {
		os.Setenv(key, "http://"+key)
		defer os.Unsetenv(key)
	}
	os.Setenv("AUTH_SERVICE_URL", "http://auth:8081")
	os.Setenv("OVERLAY_SERVICE_URL", "http://overlay:8082")

	registry, err := NewServiceRegistry()
	require.NoError(t, err)

	tests := []struct {
		name            string
		path            string
		expectedService string
		expectedBaseURL string
	}{
		{
			name:            "user overlays goes to overlay-manager",
			path:            "/api/v1/admin/user-overlays/524af57c-d33b-44f3-a14d-b1e58792ce3f",
			expectedService: "admin-user-overlays",
			expectedBaseURL: "http://overlay:8082",
		},
		{
			name:            "impersonate still goes to auth-service",
			path:            "/api/v1/admin/users/524af57c-d33b-44f3-a14d-b1e58792ce3f/impersonate",
			expectedService: "admin-users",
			expectedBaseURL: "http://auth:8081",
		},
		{
			name:            "user list still goes to auth-service",
			path:            "/api/v1/admin/users",
			expectedService: "admin-users",
			expectedBaseURL: "http://auth:8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := registry.GetServiceForPath(tt.path)
			require.NotNil(t, service)
			assert.Equal(t, tt.expectedService, service.Name)
			assert.Equal(t, tt.expectedBaseURL, service.BaseURL)
		})
	}
}

func TestMatchesPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		prefix   string
		expected bool
	}{
		{
			name:     "exact match",
			path:     "/api/v1/auth",
			prefix:   "/api/v1/auth",
			expected: true,
		},
		{
			name:     "path longer than prefix",
			path:     "/api/v1/auth/twitch/login",
			prefix:   "/api/v1/auth",
			expected: true,
		},
		{
			name:     "prefix longer than path",
			path:     "/api/v1",
			prefix:   "/api/v1/auth",
			expected: false,
		},
		{
			name:     "no match",
			path:     "/api/v1/overlays",
			prefix:   "/api/v1/auth",
			expected: false,
		},
		{
			name:     "empty prefix",
			path:     "/api/v1/auth",
			prefix:   "",
			expected: true,
		},
		{
			name:     "empty path",
			path:     "",
			prefix:   "/api/v1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPrefix(tt.path, tt.prefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}
