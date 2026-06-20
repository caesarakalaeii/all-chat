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
	"github.com/gin-gonic/gin"
)

func TestAdminOnly_NoRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no roles, got %d", resp.Code)
	}
}

func TestAdminOnly_NonAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Simulate JWTAuth setting roles
	router.Use(func(c *gin.Context) {
		c.Set("roles", []string{"user"})
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", resp.Code)
	}
}

func TestAdminOnly_AdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("roles", []string{"admin"})
		c.Next()
	})
	router.Use(AdminOnly())
	router.GET("/admin", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d", resp.Code)
	}
}

// TestJWTAuth_KidValidation tests the new KeyChain-based JWTAuth middleware.
func TestJWTAuth_KidValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret-legacy"),
		"v1",
	)

	// Generate a kid'd token
	token, err := auth.GenerateTokenWithKid(kc.LatestKid(), "user-123", "testuser", string(kc.LatestSecret()), time.Hour, false)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid kid'd token, got %d", resp.Code)
	}
}

func TestJWTAuth_ValidUserToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)

	// Legacy token (no kid) — uses legacy fallback
	token, err := auth.GenerateToken("user-123", "testuser", "test-secret", time.Hour, true)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		nil,
		"v1",
	)
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		nil,
		"v1",
	)
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestJWTAuth_AdminRouteIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)

	// Generate admin token (legacy path for back-compat)
	adminToken, err := auth.GenerateToken("admin-1", "admin", "test-secret", time.Hour, true)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	// Generate non-admin token
	userToken, err := auth.GenerateToken("user-1", "user", "test-secret", time.Hour, false)
	if err != nil {
		t.Fatalf("failed to generate user token: %v", err)
	}

	router := gin.New()
	admin := router.Group("/admin")
	admin.Use(JWTAuth(kc))
	admin.Use(AdminOnly())
	admin.GET("/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Admin token should succeed
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("admin token: expected 200, got %d", resp.Code)
	}

	// Non-admin token should be forbidden
	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("non-admin token: expected 403, got %d", resp.Code)
	}
}

// TestJWTAuth_PropagatesImpersonation verifies that an impersonation token exposes
// both the effective (target) user and the real admin to downstream handlers, so the
// moderation-service can attribute impersonated actions to the admin (ADR-0017).
func TestJWTAuth_PropagatesImpersonation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)

	// GenerateImpersonationJWT signs with the raw secret (no kid) -> legacy fallback.
	token, err := auth.GenerateImpersonationJWT("admin-1", "admin", "target-7", "victim", "tw-123", "test-secret")
	if err != nil {
		t.Fatalf("failed to generate impersonation token: %v", err)
	}

	var gotUser, gotImpersonatedBy, gotImpersonatedUser string
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		gotUser = c.GetString("user_id")
		gotImpersonatedBy = c.GetString("impersonated_by")
		gotImpersonatedUser = c.GetString("impersonated_user")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if gotUser != "target-7" {
		t.Errorf("user_id should be the effective (target) user, got %q", gotUser)
	}
	if gotImpersonatedBy != "admin-1" {
		t.Errorf("impersonated_by should be the real admin, got %q", gotImpersonatedBy)
	}
	if gotImpersonatedUser != "target-7" {
		t.Errorf("impersonated_user should be the target, got %q", gotImpersonatedUser)
	}
}

// TestJWTAuth_NormalTokenHasNoImpersonation verifies a regular token leaves the
// impersonation context keys empty (so a non-impersonated action audits as itself).
func TestJWTAuth_NormalTokenHasNoImpersonation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)
	token, err := auth.GenerateToken("user-123", "testuser", "test-secret", time.Hour, true)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var gotImpersonatedBy, gotImpersonatedUser string
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		gotImpersonatedBy = c.GetString("impersonated_by")
		gotImpersonatedUser = c.GetString("impersonated_user")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if gotImpersonatedBy != "" || gotImpersonatedUser != "" {
		t.Errorf("normal token must not set impersonation keys, got by=%q user=%q", gotImpersonatedBy, gotImpersonatedUser)
	}
}
