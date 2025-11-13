package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
)

// Mock implementations for testing
type mockUserRepository struct {
	createFunc        func(ctx context.Context, user *models.User) error
	getByIDFunc       func(ctx context.Context, id string) (*models.User, error)
	getByTwitchIDFunc func(ctx context.Context, twitchID string) (*models.User, error)
	updateFunc        func(ctx context.Context, user *models.User) error
	updateTokensFunc  func(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error
}

func (m *mockUserRepository) Create(ctx context.Context, user *models.User) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, user)
	}
	return nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) GetByTwitchID(ctx context.Context, twitchID string) (*models.User, error) {
	if m.getByTwitchIDFunc != nil {
		return m.getByTwitchIDFunc(ctx, twitchID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) Update(ctx context.Context, user *models.User) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, user)
	}
	return nil
}

func (m *mockUserRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockUserRepository) UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error {
	if m.updateTokensFunc != nil {
		return m.updateTokensFunc(ctx, userID, accessToken, refreshToken, expiresAt)
	}
	return nil
}

type mockTwitchClient struct {
	getAuthURLFunc            func(state string) string
	exchangeCodeForTokenFunc  func(code string) (*oauth.TokenResponse, error)
	getUserInfoFunc           func(accessToken string) (*oauth.UserInfo, error)
	refreshTokenFunc          func(refreshToken string) (*oauth.TokenResponse, error)
}

func (m *mockTwitchClient) GetAuthURL(state string) string {
	if m.getAuthURLFunc != nil {
		return m.getAuthURLFunc(state)
	}
	return "https://id.twitch.tv/oauth2/authorize?client_id=test&redirect_uri=http://localhost/callback&response_type=code&state=" + state
}

func (m *mockTwitchClient) ExchangeCodeForToken(code string) (*oauth.TokenResponse, error) {
	if m.exchangeCodeForTokenFunc != nil {
		return m.exchangeCodeForTokenFunc(code)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTwitchClient) GetUserInfo(accessToken string) (*oauth.UserInfo, error) {
	if m.getUserInfoFunc != nil {
		return m.getUserInfoFunc(accessToken)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTwitchClient) RefreshToken(refreshToken string) (*oauth.TokenResponse, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(refreshToken)
	}
	return nil, errors.New("not implemented")
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestAuthHandler_HandleLogin(t *testing.T) {
	tests := []struct {
		name           string
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "successful redirect to Twitch",
			wantStatusCode: http.StatusFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				location := w.Header().Get("Location")
				if location == "" {
					t.Error("HandleLogin() did not set Location header")
				}
				if len(location) < 20 {
					t.Error("HandleLogin() Location header seems invalid")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			mockClient := &mockTwitchClient{}
			handler := NewAuthHandler(mockClient, &mockUserRepository{}, "test-secret")

			router.GET("/auth/login", handler.HandleLogin)

			req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleLogin() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestAuthHandler_HandleCallback(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		mockClient     *mockTwitchClient
		mockRepo       *mockUserRepository
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successful OAuth callback - new user",
			queryParams: "?code=valid_auth_code&state=test_state",
			mockClient: &mockTwitchClient{
				exchangeCodeForTokenFunc: func(code string) (*oauth.TokenResponse, error) {
					return &oauth.TokenResponse{
						AccessToken:  "test_access_token",
						RefreshToken: "test_refresh_token",
						ExpiresIn:    14400,
					}, nil
				},
				getUserInfoFunc: func(accessToken string) (*oauth.UserInfo, error) {
					return &oauth.UserInfo{
						ID:              "123456",
						Login:           "testuser",
						DisplayName:     "TestUser",
						ProfileImageURL: "https://example.com/avatar.png",
						Email:           "test@example.com",
					}, nil
				},
			},
			mockRepo: &mockUserRepository{
				getByTwitchIDFunc: func(ctx context.Context, twitchID string) (*models.User, error) {
					return nil, repository.ErrUserNotFound
				},
				createFunc: func(ctx context.Context, user *models.User) error {
					user.ID = "550e8400-e29b-41d4-a716-446655440000"
					user.CreatedAt = time.Now()
					user.UpdatedAt = time.Now()
					return nil
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["token"] == nil {
					t.Error("HandleCallback() response missing token")
				}
				if response["user"] == nil {
					t.Error("HandleCallback() response missing user")
				}
			},
		},
		{
			name:        "successful OAuth callback - existing user",
			queryParams: "?code=valid_auth_code&state=test_state",
			mockClient: &mockTwitchClient{
				exchangeCodeForTokenFunc: func(code string) (*oauth.TokenResponse, error) {
					return &oauth.TokenResponse{
						AccessToken:  "test_access_token",
						RefreshToken: "test_refresh_token",
						ExpiresIn:    14400,
					}, nil
				},
				getUserInfoFunc: func(accessToken string) (*oauth.UserInfo, error) {
					return &oauth.UserInfo{
						ID:          "123456",
						Login:       "existinguser",
						DisplayName: "ExistingUser",
					}, nil
				},
			},
			mockRepo: &mockUserRepository{
				getByTwitchIDFunc: func(ctx context.Context, twitchID string) (*models.User, error) {
					return &models.User{
						ID:              "existing-user-id",
						TwitchID:        "123456",
						Username:        "existinguser",
						DisplayName:     "ExistingUser",
						AccessToken:     "old_token",
						RefreshToken:    "old_refresh",
						TokenExpiresAt:  time.Now().Add(1 * time.Hour),
					}, nil
				},
				updateTokensFunc: func(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error {
					return nil
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["token"] == nil {
					t.Error("HandleCallback() response missing token")
				}
			},
		},
		{
			name:           "missing code parameter",
			queryParams:    "?state=test_state",
			mockClient:     &mockTwitchClient{},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:        "failed token exchange",
			queryParams: "?code=invalid_code&state=test_state",
			mockClient: &mockTwitchClient{
				exchangeCodeForTokenFunc: func(code string) (*oauth.TokenResponse, error) {
					return nil, errors.New("invalid authorization code")
				},
			},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:        "failed user info fetch",
			queryParams: "?code=valid_code&state=test_state",
			mockClient: &mockTwitchClient{
				exchangeCodeForTokenFunc: func(code string) (*oauth.TokenResponse, error) {
					return &oauth.TokenResponse{
						AccessToken:  "test_token",
						RefreshToken: "test_refresh",
						ExpiresIn:    14400,
					}, nil
				},
				getUserInfoFunc: func(accessToken string) (*oauth.UserInfo, error) {
					return nil, errors.New("failed to fetch user info")
				},
			},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewAuthHandler(tt.mockClient, tt.mockRepo, "test-secret")

			router.GET("/auth/callback", handler.HandleCallback)

			req := httptest.NewRequest(http.MethodGet, "/auth/callback"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleCallback() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestAuthHandler_HandleRefresh(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]string
		mockClient     *mockTwitchClient
		mockRepo       *mockUserRepository
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful token refresh",
			requestBody: map[string]string{
				"refresh_token": "valid_refresh_token",
			},
			mockClient: &mockTwitchClient{
				refreshTokenFunc: func(refreshToken string) (*oauth.TokenResponse, error) {
					return &oauth.TokenResponse{
						AccessToken:  "new_access_token",
						RefreshToken: "new_refresh_token",
						ExpiresIn:    14400,
					}, nil
				},
			},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["access_token"] == nil {
					t.Error("HandleRefresh() response missing access_token")
				}
				if response["refresh_token"] == nil {
					t.Error("HandleRefresh() response missing refresh_token")
				}
			},
		},
		{
			name:           "missing refresh token",
			requestBody:    map[string]string{},
			mockClient:     &mockTwitchClient{},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "invalid refresh token",
			requestBody: map[string]string{
				"refresh_token": "invalid_token",
			},
			mockClient: &mockTwitchClient{
				refreshTokenFunc: func(refreshToken string) (*oauth.TokenResponse, error) {
					return nil, errors.New("invalid refresh token")
				},
			},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewAuthHandler(tt.mockClient, tt.mockRepo, "test-secret")

			router.POST("/auth/refresh", handler.HandleRefresh)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleRefresh() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestAuthHandler_HandleGetMe(t *testing.T) {
	validToken := "valid.jwt.token"

	tests := []struct {
		name           string
		setupAuth      func(*http.Request)
		mockRepo       *mockUserRepository
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful user fetch",
			setupAuth: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+validToken)
			},
			mockRepo: &mockUserRepository{
				getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
					return &models.User{
						ID:              "user-id-123",
						TwitchID:        "123456",
						Username:        "testuser",
						DisplayName:     "TestUser",
						ProfileImageURL: "https://example.com/avatar.png",
						Email:           "test@example.com",
					}, nil
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["username"] == nil {
					t.Error("HandleGetMe() response missing username")
				}
			},
		},
		{
			name: "missing authorization header",
			setupAuth: func(req *http.Request) {
				// Don't set header
			},
			mockRepo:       &mockUserRepository{},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "user not found",
			setupAuth: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+validToken)
			},
			mockRepo: &mockUserRepository{
				getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewAuthHandler(&mockTwitchClient{}, tt.mockRepo, "test-secret")

			// Mock auth middleware - conditionally set user_id based on test case
			router.GET("/auth/me", func(c *gin.Context) {
				// Simulate auth middleware: only set user_id if Authorization header is present and valid
				authHeader := c.GetHeader("Authorization")
				if authHeader == "Bearer "+validToken {
					c.Set("user_id", "user-id-123")
				}
				handler.HandleGetMe(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
			tt.setupAuth(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleGetMe() status = %v, want %v, body: %s", w.Code, tt.wantStatusCode, w.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestAuthHandler_HandleLogout(t *testing.T) {
	tests := []struct {
		name           string
		setupAuth      func(*http.Request)
		wantStatusCode int
	}{
		{
			name: "successful logout",
			setupAuth: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid.jwt.token")
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "logout without authorization (stateless JWT)",
			setupAuth: func(req *http.Request) {
				// Don't set header - logout should still succeed
			},
			wantStatusCode: http.StatusOK, // Changed from Unauthorized
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewAuthHandler(&mockTwitchClient{}, &mockUserRepository{}, "test-secret")

			router.POST("/auth/logout", handler.HandleLogout)

			req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
			tt.setupAuth(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleLogout() status = %v, want %v", w.Code, tt.wantStatusCode)
			}
		})
	}
}
