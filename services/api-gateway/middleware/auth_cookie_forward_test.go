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
	"github.com/gin-gonic/gin"
)

func TestAuthCookieForward_SetsHeadersFromCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAccess, gotRefresh string
	r := gin.New()
	r.Use(AuthCookieForward())
	r.POST("/x", func(c *gin.Context) {
		gotAccess = c.GetHeader("X-Access-Token")
		gotRefresh = c.GetHeader("X-Refresh-Token")
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: "acc"})
	req.AddCookie(&http.Cookie{Name: auth.CookieRefreshToken, Value: "ref"})
	r.ServeHTTP(w, req)

	if gotAccess != "acc" {
		t.Errorf("X-Access-Token=%q", gotAccess)
	}
	if gotRefresh != "ref" {
		t.Errorf("X-Refresh-Token=%q", gotRefresh)
	}
}

func TestAuthCookieForward_NoCookiesNoHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAccess, gotRefresh string
	r := gin.New()
	r.Use(AuthCookieForward())
	r.POST("/x", func(c *gin.Context) {
		gotAccess = c.GetHeader("X-Access-Token")
		gotRefresh = c.GetHeader("X-Refresh-Token")
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	r.ServeHTTP(w, req)
	if gotAccess != "" || gotRefresh != "" {
		t.Errorf("headers should be empty: access=%q refresh=%q", gotAccess, gotRefresh)
	}
}
