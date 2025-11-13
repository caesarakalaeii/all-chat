package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewTwitchClient(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		redirectURL  string
		wantErr      bool
	}{
		{
			name:         "valid configuration",
			clientID:     "test_client_id",
			clientSecret: "test_client_secret",
			redirectURL:  "http://localhost:8080/callback",
			wantErr:      false,
		},
		{
			name:         "missing client_id",
			clientID:     "",
			clientSecret: "test_client_secret",
			redirectURL:  "http://localhost:8080/callback",
			wantErr:      true,
		},
		{
			name:         "missing client_secret",
			clientID:     "test_client_id",
			clientSecret: "",
			redirectURL:  "http://localhost:8080/callback",
			wantErr:      true,
		},
		{
			name:         "missing redirect_url",
			clientID:     "test_client_id",
			clientSecret: "test_client_secret",
			redirectURL:  "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewTwitchClient(tt.clientID, tt.clientSecret, tt.redirectURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTwitchClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewTwitchClient() returned nil client")
			}
		})
	}
}

func TestTwitchClient_GetAuthURL(t *testing.T) {
	client, err := NewTwitchClient("test_id", "test_secret", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

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

func TestTwitchClient_ExchangeCodeForToken(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		checkResponse  func(*testing.T, *TokenResponse)
	}{
		{
			name: "successful token exchange",
			code: "valid_auth_code",
			mockResponse: map[string]interface{}{
				"access_token":  "test_access_token_abc123",
				"refresh_token": "test_refresh_token_xyz789",
				"expires_in":    14400,
				"token_type":    "bearer",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			checkResponse: func(t *testing.T, resp *TokenResponse) {
				if resp.AccessToken != "test_access_token_abc123" {
					t.Errorf("AccessToken = %v, want test_access_token_abc123", resp.AccessToken)
				}
				if resp.RefreshToken != "test_refresh_token_xyz789" {
					t.Errorf("RefreshToken = %v, want test_refresh_token_xyz789", resp.RefreshToken)
				}
				if resp.ExpiresIn != 14400 {
					t.Errorf("ExpiresIn = %v, want 14400", resp.ExpiresIn)
				}
			},
		},
		{
			name:           "empty code",
			code:           "",
			mockResponse:   nil,
			mockStatusCode: http.StatusOK,
			wantErr:        true,
		},
		{
			name: "twitch API error",
			code: "invalid_code",
			mockResponse: map[string]interface{}{
				"status":  400,
				"message": "Invalid authorization code",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
		},
		{
			name:           "network error (500)",
			code:           "valid_code",
			mockResponse:   nil,
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create client with mock server URL
			client := &TwitchClient{
				clientID:     "test_id",
				clientSecret: "test_secret",
				redirectURL:  "http://localhost:8080/callback",
				httpClient:   &http.Client{Timeout: 10 * time.Second},
				tokenURL:     server.URL, // Override with mock server
			}

			resp, err := client.ExchangeCodeForToken(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExchangeCodeForToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestTwitchClient_GetUserInfo(t *testing.T) {
	tests := []struct {
		name           string
		accessToken    string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		checkResponse  func(*testing.T, *UserInfo)
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
						"email":             "test@example.com",
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			checkResponse: func(t *testing.T, info *UserInfo) {
				if info.ID != "123456789" {
					t.Errorf("ID = %v, want 123456789", info.ID)
				}
				if info.Login != "testuser" {
					t.Errorf("Login = %v, want testuser", info.Login)
				}
				if info.DisplayName != "TestUser" {
					t.Errorf("DisplayName = %v, want TestUser", info.DisplayName)
				}
				if info.Email != "test@example.com" {
					t.Errorf("Email = %v, want test@example.com", info.Email)
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

			// Create client with mock server URL
			client := &TwitchClient{
				clientID:     "test_id",
				clientSecret: "test_secret",
				redirectURL:  "http://localhost:8080/callback",
				httpClient:   &http.Client{Timeout: 10 * time.Second},
				userInfoURL:  server.URL, // Override with mock server
			}

			info, err := client.GetUserInfo(tt.accessToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkResponse != nil {
				tt.checkResponse(t, info)
			}
		})
	}
}

func TestTwitchClient_RefreshToken(t *testing.T) {
	tests := []struct {
		name           string
		refreshToken   string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		checkResponse  func(*testing.T, *TokenResponse)
	}{
		{
			name:         "successful token refresh",
			refreshToken: "valid_refresh_token",
			mockResponse: map[string]interface{}{
				"access_token":  "new_access_token_abc123",
				"refresh_token": "new_refresh_token_xyz789",
				"expires_in":    14400,
				"token_type":    "bearer",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			checkResponse: func(t *testing.T, resp *TokenResponse) {
				if resp.AccessToken != "new_access_token_abc123" {
					t.Errorf("AccessToken = %v, want new_access_token_abc123", resp.AccessToken)
				}
				if resp.RefreshToken != "new_refresh_token_xyz789" {
					t.Errorf("RefreshToken = %v, want new_refresh_token_xyz789", resp.RefreshToken)
				}
			},
		},
		{
			name:           "empty refresh token",
			refreshToken:   "",
			mockResponse:   nil,
			mockStatusCode: http.StatusOK,
			wantErr:        true,
		},
		{
			name:         "invalid refresh token",
			refreshToken: "invalid_token",
			mockResponse: map[string]interface{}{
				"status":  400,
				"message": "Invalid refresh token",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create client with mock server URL
			client := &TwitchClient{
				clientID:     "test_id",
				clientSecret: "test_secret",
				redirectURL:  "http://localhost:8080/callback",
				httpClient:   &http.Client{Timeout: 10 * time.Second},
				tokenURL:     server.URL, // Override with mock server
			}

			resp, err := client.RefreshToken(tt.refreshToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefreshToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}
