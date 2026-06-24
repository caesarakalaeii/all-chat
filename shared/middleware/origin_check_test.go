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
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOriginCheck_AllowsAllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("Origin", "https://allch.at")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("want 200 got %d", w.Code)
	}
}

func TestOriginCheck_RejectsDisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("want 403 got %d", w.Code)
	}
}

func TestOriginCheck_AllowsAbsentOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil) // no Origin/Referer
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("absent Origin should be allowed (non-browser), got %d", w.Code)
	}
}

func TestOriginCheck_SkipsGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET should be allowed regardless of Origin, got %d", w.Code)
	}
}
