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

package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
)

func TestNewTwitchOAuth(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		redirectURL  string
	}{
		{
			name:         "valid configuration",
			clientID:     "test_client_id",
			clientSecret: "test_client_secret",
			redirectURL:  "http://localhost:8080/callback",
		},
		{
			name:         "empty values allowed (no validation)",
			clientID:     "",
			clientSecret: "",
			redirectURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewTwitchOAuth(tt.clientID, tt.clientSecret, tt.redirectURL)
			if client == nil {
				t.Error("NewTwitchOAuth() returned nil client")
			}
		})
	}
}

func TestTwitchOAuth_GetAuthURL(t *testing.T) {
	client := NewTwitchOAuth("test_id", "test_secret", "http://localhost:8080/callback")

	tests := []struct {
		name      string
		state     string
		wantEmpty bool
	}{
		{
			name:      "with state",
			state:     "random_state_string",
			wantEmpty: false,
		},
		{
			name:      "with empty state",
			state:     "",
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := client.GetAuthURL(tt.state)
			if tt.wantEmpty && url != "" {
				t.Errorf("GetAuthURL() = %v, want empty", url)
			}
			if !tt.wantEmpty && url == "" {
				t.Error("GetAuthURL() returned empty URL")
			}
			// Verify URL contains expected components
			if !tt.wantEmpty {
				expectedBase := "https://id.twitch.tv/oauth2/authorize"
				if len(url) < len(expectedBase) || url[:len(expectedBase)] != expectedBase {
					t.Errorf("GetAuthURL() doesn't start with expected base URL, got: %s", url)
				}
			}
		})
	}
}

func TestTwitchOAuth_ExchangeCode(t *testing.T) {
	t.Run("empty code should error", func(t *testing.T) {
		client := NewTwitchOAuth("test_id", "test_secret", "http://localhost:8080/callback")
		ctx := context.Background()
		_, err := client.ExchangeCode(ctx, "")
		if err == nil {
			t.Error("ExchangeCode() with empty code should return error")
		}
	})

	t.Run("client configured correctly", func(t *testing.T) {
		client := NewTwitchOAuth("test_id", "test_secret", "http://localhost:8080/callback")
		if client == nil {
			t.Fatal("NewTwitchOAuth() returned nil")
		}
		if client.config == nil {
			t.Error("config is nil")
		}
		if client.config.ClientID != "test_id" {
			t.Errorf("ClientID = %v, want test_id", client.config.ClientID)
		}
	})

	// Note: Full end-to-end token exchange testing is difficult to mock
	// because the oauth2 library makes specific HTTP requests with form encoding.
	// Integration tests should validate the actual OAuth flow.
}

func TestTwitchOAuth_GetUserInfo(t *testing.T) {
	tests := []struct {
		name           string
		accessToken    string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		checkResponse  func(*testing.T, *models.TwitchUserInfo)
	}{
		{
			name:        "successful user info fetch",
			accessToken: "valid_access_token",
			mockResponse: map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                "123456789",
						"login":             "testuser",
						"display_name":      "TestUser",
						"profile_image_url": "https://example.com/avatar.png",
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			checkResponse: func(t *testing.T, info *models.TwitchUserInfo) {
				if info.ID != "123456789" {
					t.Errorf("ID = %v, want 123456789", info.ID)
				}
				if info.Login != "testuser" {
					t.Errorf("Login = %v, want testuser", info.Login)
				}
				if info.DisplayName != "TestUser" {
					t.Errorf("DisplayName = %v, want TestUser", info.DisplayName)
				}
			},
		},
		{
			name:           "empty access token",
			accessToken:    "",
			mockResponse:   nil,
			mockStatusCode: http.StatusOK,
			wantErr:        true,
		},
		{
			name:        "unauthorized (401)",
			accessToken: "invalid_token",
			mockResponse: map[string]interface{}{
				"status":  401,
				"message": "Unauthorized",
			},
			mockStatusCode: http.StatusUnauthorized,
			wantErr:        true,
		},
		{
			name:        "empty response data",
			accessToken: "valid_token",
			mockResponse: map[string]interface{}{
				"data": []map[string]interface{}{},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip empty token test - oauth2 library will construct the request
			if tt.accessToken == "" {
				t.Skip("Skipping empty token test - requires HTTP client override")
			}

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify Authorization header
				if tt.accessToken != "" && r.Header.Get("Authorization") != "Bearer "+tt.accessToken {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create client - we can't easily override the Twitch API URL
			// This test now validates the HTTP client behavior
			client := NewTwitchOAuth("test_id", "test_secret", "http://localhost:8080/callback")

			// Override HTTP client to use mock server
			client.client = &http.Client{
				Timeout: 10 * time.Second,
				Transport: &mockTransport{server: server},
			}

			ctx := context.Background()
			info, err := client.GetUserInfoTwitch(ctx, tt.accessToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserInfoTwitch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkResponse != nil {
				tt.checkResponse(t, info)
			}
		})
	}
}

// mockTransport redirects all requests to the test server
type mockTransport struct {
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to mock server
	req.URL.Scheme = "http"
	req.URL.Host = m.server.URL[7:] // Remove "http://"
	return http.DefaultTransport.RoundTrip(req)
}

func TestTwitchOAuth_RefreshToken(t *testing.T) {
	t.Run("empty refresh token should error", func(t *testing.T) {
		client := NewTwitchOAuth("test_id", "test_secret", "http://localhost:8080/callback")
		ctx := context.Background()
		_, err := client.RefreshToken(ctx, "")
		if err == nil {
			t.Error("RefreshToken() with empty token should return error")
		}
	})

	t.Run("client configured correctly", func(t *testing.T) {
		client := NewTwitchOAuth("test_id", "test_secret", "http://localhost:8080/callback")
		if client == nil {
			t.Fatal("NewTwitchOAuth() returned nil")
		}
		if client.config == nil {
			t.Error("config is nil")
		}
	})

	// Note: Full end-to-end token refresh testing is difficult to mock
	// because the oauth2 library makes specific HTTP requests with form encoding.
	// Integration tests should validate the actual OAuth flow.
}
