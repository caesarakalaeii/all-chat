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

func TestJWTAuth_ValidUserToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	token, err := auth.GenerateToken("user-123", "testuser", secret, time.Hour, true)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	router := gin.New()
	router.Use(JWTAuth(secret))
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
	router := gin.New()
	router.Use(JWTAuth("test-secret"))
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
	router := gin.New()
	router.Use(JWTAuth("test-secret"))
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
	secret := "test-secret"

	// Generate admin token
	adminToken, err := auth.GenerateToken("admin-1", "admin", secret, time.Hour, true)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	// Generate non-admin token
	userToken, err := auth.GenerateToken("user-1", "user", secret, time.Hour, false)
	if err != nil {
		t.Fatalf("failed to generate user token: %v", err)
	}

	router := gin.New()
	admin := router.Group("/admin")
	admin.Use(JWTAuth(secret))
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
