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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap/zaptest"
)

// testUserKeyChain builds a single-key KeyChain suitable for tests.
func testUserKeyChain(secret string) *auth.KeyChain {
	return auth.NewKeyChain(
		map[string][]byte{"v1": []byte(secret)},
		[]byte(secret),
		"v1",
	)
}

// TestAuthHandlerCreation verifies the auth handler can be created
func TestAuthHandlerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	redisClient := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})

	twitchOAuth := oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback")
	youtubeOAuth := oauth.NewYouTubeOAuth("test-id", "test-secret", "http://localhost/callback")

	// Create a mock user repository
	userRepo := &repository.UserRepository{} // This will fail DB operations but that's ok for construction test

	handler := NewAuthHandler(
		twitchOAuth,
		youtubeOAuth,
		nil, // kickOAuth
		userRepo,
		redisClient,
		testUserKeyChain("test-jwt-secret"),
		24,
		logger,
	)

	if handler == nil {
		t.Fatal("NewAuthHandler returned nil")
	}

	if handler.jwtExpiry != 24*time.Hour {
		t.Errorf("jwtExpiry = %v, want 24h", handler.jwtExpiry)
	}
}

// TestAuthHandlerLogout covers the logout endpoint's contract: a request with no usable token is
// refused, and a request with one has that token blacklisted so it cannot be replayed.
//
// It runs against miniredis rather than a real server on localhost:6379. The previous version
// dialled the real port, which meant it passed only on a developer machine that happened to have
// Redis up, and failed every night in CI (the nightly job runs without -short and has no Redis
// service) — a permanently red job tells nobody anything. It also asserted only the status code,
// so the blacklist write, which is the entire security purpose of logout, went unverified.
func TestAuthHandlerLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newLogoutHandler := func(t *testing.T) (*AuthHandler, *miniredis.Miniredis) {
		t.Helper()
		mr := miniredis.RunT(t)
		return NewAuthHandler(
			oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback"),
			nil,
			nil, // kickOAuth
			&repository.UserRepository{},
			redis.NewClient(&redis.Options{Addr: mr.Addr()}),
			testUserKeyChain("test-jwt-secret"),
			24,
			zaptest.NewLogger(t),
		), mr
	}

	t.Run("a request carrying no token is refused", func(t *testing.T) {
		for _, header := range []struct{ name, value string }{
			{"absent", ""},
			{"not a bearer token", "InvalidFormat"},
		} {
			t.Run(header.name, func(t *testing.T) {
				handler, mr := newLogoutHandler(t)
				router := gin.New()
				router.POST("/auth/logout", handler.HandleLogout)

				req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
				if header.value != "" {
					req.Header.Set("Authorization", header.value)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, want 401, body: %s", w.Code, w.Body.String())
				}
				if keys := mr.Keys(); len(keys) != 0 {
					t.Errorf("a refused logout must write nothing, got %v", keys)
				}
			})
		}
	})

	t.Run("a bearer token is blacklisted for as long as it could still be valid", func(t *testing.T) {
		handler, mr := newLogoutHandler(t)
		router := gin.New()
		router.POST("/auth/logout", handler.HandleLogout)

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer valid.jwt.token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
		}
		// The blacklist entry is what makes logout mean anything: shared/middleware checks this key
		// on every request, so without it a logged-out JWT keeps working until it expires.
		if got, err := mr.Get("blacklist:valid.jwt.token"); err != nil {
			t.Errorf("logout must blacklist the presented token: %v", err)
		} else if got != "1" {
			t.Errorf("blacklist value = %q, want \"1\"", got)
		}
		// A TTL shorter than the JWT would let the token come back to life.
		if ttl := mr.TTL("blacklist:valid.jwt.token"); ttl != 24*time.Hour {
			t.Errorf("blacklist TTL = %v, want the JWT expiry (24h)", ttl)
		}
	})

	// The gateway forwards the access token in X-Access-Token; Authorization is backward compat.
	t.Run("the gateway's X-Access-Token header is honoured", func(t *testing.T) {
		handler, mr := newLogoutHandler(t)
		router := gin.New()
		router.POST("/auth/logout", handler.HandleLogout)

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.Header.Set("X-Access-Token", "from.the.gateway")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
		}
		if _, err := mr.Get("blacklist:from.the.gateway"); err != nil {
			t.Errorf("the gateway-forwarded token must be blacklisted: %v", err)
		}
	})

	// Audit H3: logging out must also kill the refresh token, or the session can be minted again.
	t.Run("a presented refresh token is revoked too", func(t *testing.T) {
		handler, mr := newLogoutHandler(t)
		router := gin.New()
		router.POST("/auth/logout", handler.HandleLogout)

		const refreshToken = "refresh-token-value"
		rtKey := "refresh_token:" + refreshTokenHash(refreshToken)
		if err := mr.Set(rtKey, "user-1"); err != nil {
			t.Fatalf("seed refresh token: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer valid.jwt.token")
		req.Header.Set("X-Refresh-Token", refreshToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
		}
		if mr.Exists(rtKey) {
			t.Error("the refresh token family entry must be gone, or logout is reversible")
		}
	})

	// A personal access token (ADR-0050) is not a session. Blacklisting one would write the
	// plaintext secret into Redis under "blacklist:<raw-token>" AND achieve nothing, because
	// the PAT path never consults the blacklist — revocation is api_tokens.revoked_at, read
	// live. So logout refuses it rather than pretending to have logged something out.
	t.Run("a personal access token is neither blacklisted nor accepted", func(t *testing.T) {
		handler, mr := newLogoutHandler(t)
		router := gin.New()
		router.POST("/auth/logout", handler.HandleLogout)

		const pat = sharedmiddleware.APITokenPrefix + "do-not-blacklist-me"
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+pat)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 for a PAT-authenticated logout", w.Code)
		}
		for _, key := range mr.Keys() {
			if strings.Contains(key, pat) {
				t.Fatalf("the plaintext token reached Redis as key %q", key)
			}
		}
	})

	// Redis is the only place the blacklist can live, so a write failure must fail the logout
	// rather than reporting success and leaving the token usable.
	t.Run("an unreachable Redis fails the logout instead of lying", func(t *testing.T) {
		handler, mr := newLogoutHandler(t)
		mr.Close()
		router := gin.New()
		router.POST("/auth/logout", handler.HandleLogout)

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer valid.jwt.token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500 when the blacklist cannot be written", w.Code)
		}
	})
}

// TestAuthHandlerGetMe tests the /me endpoint behavior without auth
func TestAuthHandlerGetMe_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gin.SetMode(gin.TestMode)

	logger := zaptest.NewLogger(t)
	redisClient := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})

	twitchOAuth := oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback")
	userRepo := &repository.UserRepository{}

	handler := NewAuthHandler(
		twitchOAuth,
		nil,
		nil, // kickOAuth
		userRepo,
		redisClient,
		testUserKeyChain("test-jwt-secret"),
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
