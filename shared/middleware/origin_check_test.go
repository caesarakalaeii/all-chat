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

func TestOriginAllowed_WildcardStarSuffix_MatchesExtensionOrigin(t *testing.T) {
	// M4: moz-extension://* (WS-format wildcard) must match a real extension origin.
	allowed := []string{"moz-extension://*", "https://allch.at"}
	if !OriginAllowed(allowed, "moz-extension://7d4f8b2e-1234-abcd") {
		t.Error("moz-extension://* should match moz-extension://<uuid>")
	}
}

func TestOriginAllowed_WildcardSlashStarSuffix_MatchesExtensionOrigin(t *testing.T) {
	// M4: moz-extension:///* (CORS-format wildcard) must match a real extension origin.
	allowed := []string{"moz-extension:///*", "https://allch.at"}
	if !OriginAllowed(allowed, "moz-extension://7d4f8b2e-1234-abcd") {
		t.Error("moz-extension:///* should match moz-extension://<uuid>")
	}
}

func TestOriginAllowed_ExactMatch(t *testing.T) {
	allowed := []string{"https://allch.at", "https://app.allch.at"}
	if !OriginAllowed(allowed, "https://allch.at") {
		t.Error("exact match should return true")
	}
	if !OriginAllowed(allowed, "https://app.allch.at") {
		t.Error("exact match should return true")
	}
}

func TestOriginAllowed_UnrelatedOriginRejected(t *testing.T) {
	allowed := []string{"https://allch.at", "moz-extension://*"}
	if OriginAllowed(allowed, "https://evil.example") {
		t.Error("unrelated origin should be rejected")
	}
}

func TestOriginAllowed_ChromeExtensionWildcard(t *testing.T) {
	allowed := []string{"chrome-extension://*"}
	if !OriginAllowed(allowed, "chrome-extension://abcdef123456") {
		t.Error("chrome-extension://* should match chrome-extension://<id>")
	}
	if OriginAllowed(allowed, "moz-extension://abcdef123456") {
		t.Error("chrome-extension://* should NOT match moz-extension origin")
	}
}

func TestOriginAllowed_StandaloneStarAllowsAll(t *testing.T) {
	allowed := []string{"*"}
	if !OriginAllowed(allowed, "https://anything.com") {
		t.Error("* should allow all origins")
	}
	if !OriginAllowed(allowed, "moz-extension://abc") {
		t.Error("* should allow extension origins")
	}
}

func TestOriginAllowed_EmptyListRejectsAll(t *testing.T) {
	if OriginAllowed([]string{}, "https://allch.at") {
		t.Error("empty allowlist should reject all origins")
	}
	if OriginAllowed(nil, "https://allch.at") {
		t.Error("nil allowlist should reject all origins")
	}
}

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

// TestOriginAllowed_HostBoundary guards audit #16: a host-wildcard allowlist
// entry must not match a different host that merely shares a prefix. Extension
// scheme-wildcards (prefix ends in "://") and legitimate path boundaries must
// still match.
func TestOriginAllowed_HostBoundary(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		origin  string
		want    bool
	}{
		{"star suffix must not cross host boundary", []string{"https://allch.at*"}, "https://allch.at.evil.com", false},
		{"slashstar suffix must not cross host boundary", []string{"https://allch.at/*"}, "https://allch.at.evil.com", false},
		{"slashstar still matches bare origin", []string{"https://allch.at/*"}, "https://allch.at", true},
		{"slashstar still matches path boundary", []string{"https://allch.at/*"}, "https://allch.at/app", true},
		{"extension scheme wildcard still matches", []string{"moz-extension://*"}, "moz-extension://abc-123", true},
	}
	for _, tc := range cases {
		if got := OriginAllowed(tc.allowed, tc.origin); got != tc.want {
			t.Errorf("%s: OriginAllowed(%v, %q) = %v, want %v", tc.name, tc.allowed, tc.origin, got, tc.want)
		}
	}
}
