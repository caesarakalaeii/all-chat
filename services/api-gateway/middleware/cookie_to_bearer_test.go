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

	"github.com/caesar/all-chat/shared/auth"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
)

func TestCookieToBearer_SetsAuthorizationFromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: "jwt-from-cookie"})
	r.ServeHTTP(w, req)
	if gotAuth != "Bearer jwt-from-cookie" {
		t.Errorf("Authorization=%q", gotAuth)
	}
}

func TestCookieToBearer_DoesNotOverwriteExistingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer explicit")
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: "jwt-from-cookie"})
	r.ServeHTTP(w, req)
	if gotAuth != "Bearer explicit" {
		t.Errorf("Authorization overwritten: %q", gotAuth)
	}
}

func TestCookieToBearer_NoCookieNoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)
	if gotAuth != "" {
		t.Errorf("Authorization should be empty, got %q", gotAuth)
	}
}

// A personal access token is a header credential for non-browser clients, so it is
// never promoted from the cookie: a planted cookie would otherwise make a victim's
// browser act as the attacker's account (ADR-0051).
func TestCookieToBearer_IgnoresPersonalAccessTokenCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: sharedmiddleware.APITokenPrefix + "planted"})
	r.ServeHTTP(w, req)
	if gotAuth != "" {
		t.Errorf("a PAT cookie was promoted to a Bearer header: %q", gotAuth)
	}
}
