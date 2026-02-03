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
				assert.Len(t, sr.Services, 6) // 3 base services + 3 admin routes
				assert.NotNil(t, sr.Services["auth-service"])
				assert.NotNil(t, sr.Services["overlay-manager"])
				assert.NotNil(t, sr.Services["emote-service"])
				assert.NotNil(t, sr.Services["admin-users"])
				assert.NotNil(t, sr.Services["admin-overlays"])
				assert.NotNil(t, sr.Services["admin-sources"])
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
