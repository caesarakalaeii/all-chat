package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap/zaptest"
)

// TestAuthHandlerCreation verifies the auth handler can be created
func TestAuthHandlerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	twitchOAuth := oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback")
	youtubeOAuth := oauth.NewYouTubeOAuth("test-id", "test-secret", "http://localhost/callback")

	// Create a mock user repository
	userRepo := &repository.UserRepository{} // This will fail DB operations but that's ok for construction test

	handler := NewAuthHandler(
		twitchOAuth,
		youtubeOAuth,
		userRepo,
		redisClient,
		"test-jwt-secret",
		24,
		logger,
	)

	if handler == nil {
		t.Fatal("NewAuthHandler returned nil")
	}

	if handler.jwtSecret != "test-jwt-secret" {
		t.Errorf("jwtSecret = %v, want test-jwt-secret", handler.jwtSecret)
	}

	if handler.jwtExpiry != 24*time.Hour {
		t.Errorf("jwtExpiry = %v, want 24h", handler.jwtExpiry)
	}
}

// TestAuthHandlerLogout tests the logout endpoint
func TestAuthHandlerLogout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
	}{
		{
			name:           "logout without authorization",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "logout with invalid authorization format",
			authHeader:     "InvalidFormat",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "logout with authorization (requires Redis)",
			authHeader:     "Bearer valid.jwt.token",
			wantStatusCode: http.StatusOK, // Will fail if Redis not available, but structure is correct
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			redisClient := redis.NewClient(&redis.Options{
				Addr: "localhost:6379",
			})

			twitchOAuth := oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback")
			userRepo := &repository.UserRepository{}

			handler := NewAuthHandler(
				twitchOAuth,
				nil,
				userRepo,
				redisClient,
				"test-jwt-secret",
				24,
				logger,
			)

			router := gin.New()
			router.POST("/auth/logout", handler.HandleLogout)

			req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Skip Redis-dependent tests in short mode
			if testing.Short() && tt.name == "logout with authorization (requires Redis)" {
				t.Skip("Skipping Redis-dependent test in short mode")
			}

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleLogout() status = %v, want %v, body: %s", w.Code, tt.wantStatusCode, w.Body.String())
			}
		})
	}
}

// TestAuthHandlerGetMe tests the /me endpoint behavior without auth
func TestAuthHandlerGetMe_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gin.SetMode(gin.TestMode)

	logger := zaptest.NewLogger(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	twitchOAuth := oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback")
	userRepo := &repository.UserRepository{}

	handler := NewAuthHandler(
		twitchOAuth,
		nil,
		userRepo,
		redisClient,
		"test-jwt-secret",
		24,
		logger,
	)

	router := gin.New()
	router.GET("/auth/me", handler.HandleGetMe)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return unauthorized without user_id in context
	if w.Code != http.StatusUnauthorized {
		t.Errorf("HandleGetMe() status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

// TestGenerateRandomString tests the random string generation helper
func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"short string", 8},
		{"medium string", 16},
		{"long string", 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, err := generateRandomString(tt.length)
			if err != nil {
				t.Errorf("generateRandomString() error = %v", err)
				return
			}

			if len(str) == 0 {
				t.Error("generateRandomString() returned empty string")
			}

			// Second call should produce different string
			str2, err := generateRandomString(tt.length)
			if err != nil {
				t.Errorf("generateRandomString() error = %v", err)
				return
			}

			if str == str2 {
				t.Error("generateRandomString() produced identical strings (should be random)")
			}
		})
	}
}

// MockAuthUserRepository implements repository.UserRepository for testing
type MockAuthUserRepository struct {
	CreateFunc        func(ctx context.Context, user *models.User) error
	GetByIDFunc       func(ctx context.Context, id string) (*models.User, error)
	GetByTwitchIDFunc func(ctx context.Context, twitchID string) (*models.User, error)
	GetByGoogleIDFunc func(ctx context.Context, googleID string) (*models.User, error)
	UpdateFunc        func(ctx context.Context, user *models.User) error
	DeleteFunc        func(ctx context.Context, id string) error
	UpdateTokensFunc  func(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error
}

func (m *MockAuthUserRepository) Create(ctx context.Context, user *models.User) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, user)
	}
	return nil
}

func (m *MockAuthUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, repository.ErrUserNotFound
}

func (m *MockAuthUserRepository) GetByTwitchID(ctx context.Context, twitchID string) (*models.User, error) {
	if m.GetByTwitchIDFunc != nil {
		return m.GetByTwitchIDFunc(ctx, twitchID)
	}
	return nil, repository.ErrUserNotFound
}

func (m *MockAuthUserRepository) GetByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	if m.GetByGoogleIDFunc != nil {
		return m.GetByGoogleIDFunc(ctx, googleID)
	}
	return nil, repository.ErrUserNotFound
}

func (m *MockAuthUserRepository) Update(ctx context.Context, user *models.User) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, user)
	}
	return nil
}

func (m *MockAuthUserRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockAuthUserRepository) UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error {
	if m.UpdateTokensFunc != nil {
		return m.UpdateTokensFunc(ctx, userID, accessToken, refreshToken, expiresAt)
	}
	return nil
}
