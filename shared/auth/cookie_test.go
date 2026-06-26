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

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSetAuthCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	SetAuthCookies(c, "access-jwt", "refresh-jwt", time.Hour, 14*24*time.Hour)

	cookies := w.Result().Cookies()
	var gotAccess, gotRefresh bool
	for _, ck := range cookies {
		switch ck.Name {
		case CookieAccessToken:
			gotAccess = true
			if ck.Value != "access-jwt" {
				t.Errorf("access value=%s", ck.Value)
			}
			if !ck.HttpOnly {
				t.Error("access not HttpOnly")
			}
			if !ck.Secure {
				t.Error("access not Secure")
			}
			if ck.SameSite != http.SameSiteLaxMode {
				t.Error("access not Lax")
			}
			if ck.Path != "/" {
				t.Errorf("access path=%s", ck.Path)
			}
		case CookieRefreshToken:
			gotRefresh = true
			if ck.Value != "refresh-jwt" {
				t.Errorf("refresh value=%s", ck.Value)
			}
			if ck.Path != "/api/v1/auth/" {
				t.Errorf("refresh path=%s", ck.Path)
			}
		}
	}
	if !gotAccess {
		t.Error("access cookie missing")
	}
	if !gotRefresh {
		t.Error("refresh cookie missing")
	}
}

func TestClearAuthCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	ClearAuthCookies(c)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("want 2 cleared cookies, got %d", len(cookies))
	}
	for _, ck := range cookies {
		if ck.MaxAge != -1 && ck.Value != "" {
			t.Errorf("cookie %s not cleared", ck.Name)
		}
		if ck.Name == CookieRefreshToken && ck.Path != "/api/v1/auth/" {
			t.Errorf("refresh clear path=%s", ck.Path)
		}
	}
}
