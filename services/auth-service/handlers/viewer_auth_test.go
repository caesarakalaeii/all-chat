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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TestSanitizeRedirectPath_BackslashBypass (audit M1) verifies that a path
// containing a backslash is rejected. Browsers normalize \ → /, so
// /\evil.com becomes //evil.com (cross-origin). Pre-fix this passed through
// and was an open-redirect vector.
func TestSanitizeRedirectPath_BackslashBypass(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string // expected return value ("" = rejected)
	}{
		{
			name: "backslash after slash rejected",
			path: `/\evil.com`,
			want: "",
		},
		{
			name: "valid relative path allowed",
			path: "/dashboard",
			want: "/dashboard",
		},
		{
			name: "protocol-relative double-slash rejected",
			path: "//evil.com",
			want: "",
		},
		{
			name: "backslash mid-path rejected",
			path: `/dashboard\evil.com`,
			want: "",
		},
		{
			name: "scheme rejected",
			path: "https://evil.com",
			want: "",
		},
		{
			name: "empty path returns empty",
			path: "",
			want: "",
		},
		{
			name: "root path allowed",
			path: "/",
			want: "/",
		},
		{
			name: "nested relative path allowed",
			path: "/overlay/abc-123/config",
			want: "/overlay/abc-123/config",
		},
		{
			name: "query-like relative path allowed",
			path: "/dashboard?source_added=twitch",
			want: "/dashboard?source_added=twitch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeRedirectPath(tt.path)
			if got != tt.want {
				t.Errorf("sanitizeRedirectPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestViewerCallbackTombstone_Replay (audit I2) verifies the idempotency
// tombstone round-trip that protects viewer OAuth callbacks against duplicate
// delivery (iOS Safari prefetch, Google multi-code). On the first callback,
// redirectWithTombstone stores the final redirect URL under <stateKey>:used
// (60s). When a duplicate arrives, the state was already consumed by GetDel;
// replayCallbackTombstone must replay the original redirect instead of
// hard-failing with "Invalid or expired state".
func TestViewerCallbackTombstone_Replay(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	h := &ViewerAuthHandler{
		redis:       rdb,
		logger:      zap.NewNop(),
		frontendURL: "https://app.test",
	}
	gin.SetMode(gin.TestMode)

	stateKey := "viewer_oauth_state:twitch:dup-state"
	originalRedirect := "https://app.test/chat/auth-success?code=abc&streamer=s"

	// First callback: store the tombstone + redirect.
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/callback?code=x&state=dup-state", nil)
	h.redirectWithTombstone(c1, stateKey, originalRedirect)
	if w1.Code != http.StatusFound {
		t.Fatalf("first callback: want 302, got %d", w1.Code)
	}
	if loc := w1.Header().Get("Location"); loc != originalRedirect {
		t.Fatalf("first callback: want redirect to %q, got %q", originalRedirect, loc)
	}
	// Tombstone key must exist.
	if !mr.Exists(stateKey + ":used") {
		t.Fatal("tombstone key not stored")
	}

	// Duplicate callback: state already consumed (GetDel would miss).
	// replayCallbackTombstone must replay the original redirect.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/callback?code=x&state=dup-state", nil)
	replayed := h.replayCallbackTombstone(c2, stateKey)
	if !replayed {
		t.Fatal("duplicate callback: replayCallbackTombstone returned false (no tombstone found)")
	}
	if w2.Code != http.StatusFound {
		t.Fatalf("duplicate callback: want 302 replay, got %d", w2.Code)
	}
	if loc := w2.Header().Get("Location"); loc != originalRedirect {
		t.Fatalf("duplicate callback: want replay redirect to %q, got %q", originalRedirect, loc)
	}
}

// TestViewerCallbackTombstone_NoTombstoneFallsThrough (audit I2) verifies that
// when no tombstone exists (genuinely new/invalid state, not a duplicate),
// replayCallbackTombstone returns false so the caller proceeds to the
// "Invalid or expired state" error path.
func TestViewerCallbackTombstone_NoTombstoneFallsThrough(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	h := &ViewerAuthHandler{
		redis:       rdb,
		logger:      zap.NewNop(),
		frontendURL: "https://app.test",
	}
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/callback?code=x&state=never-seen", nil)
	if h.replayCallbackTombstone(c, "viewer_oauth_state:twitch:never-seen") {
		t.Fatal("want false when no tombstone exists, got true")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("want no redirect (200 default), got %d", w.Code)
	}
}
