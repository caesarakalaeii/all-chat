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

func TestServiceJWTAuthMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ServiceJWTAuth("test-secret"))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.Code)
	}
}

func TestServiceJWTAuthDisallowedService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	token, err := auth.GenerateServiceJWT("youtube-listener", secret, time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	router := gin.New()
	router.Use(ServiceJWTAuth(secret, "source-manager"))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.Code)
	}
}

func TestServiceJWTAuthAllowedService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	token, err := auth.GenerateServiceJWT("source-manager", secret, time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	router := gin.New()
	router.Use(ServiceJWTAuth(secret, "source-manager"))
	router.GET("/protected", func(c *gin.Context) {
		svc, _ := c.Get("service_name")
		c.JSON(http.StatusOK, gin.H{"service": svc})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
}
