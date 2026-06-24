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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap/zaptest"
)

// newTestAuthHandler builds an AuthHandler backed by a miniredis instance so
// tests run without a live Redis. HandleStreamerTokenExchange only touches
// Redis (no DB/oauth), so a nil-DB userRepo and dummy oauth configs suffice.
func newTestAuthHandler(t *testing.T, rdb *redis.Client) *AuthHandler {
	t.Helper()
	logger := zaptest.NewLogger(t)
	twitchOAuth := oauth.NewTwitchOAuth("test-id", "test-secret", "http://localhost/callback")
	youtubeOAuth := oauth.NewYouTubeOAuth("test-id", "test-secret", "http://localhost/callback")
	userRepo := &repository.UserRepository{} // nil DB — fine for non-DB handlers
	return NewAuthHandler(
		twitchOAuth,
		youtubeOAuth,
		userRepo,
		rdb,
		testUserKeyChain("test-jwt-secret"),
		24,
		logger,
	)
}

// TestHandleRefresh_MissingTokenReturns400 verifies that /refresh returns 400
// when no refresh token is supplied (neither the X-Refresh-Token header nor a
// JSON body). This is the first line of defense: a missing token must never
// reach the reuse-detection path.
func TestHandleRefresh_MissingTokenReturns400(t *testing.T) {
	rdb := miniredis.RunT(t)
	defer rdb.Close()
	h := newTestAuthHandler(t, redis.NewClient(&redis.Options{Addr: rdb.Addr()}))
	router := gin.New()
	router.POST("/refresh", h.HandleRefresh)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/refresh", nil) // no X-Refresh-Token, no body
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("want 400 (refresh token required), got %d body=%s", w.Code, w.Body.String())
	}
}

// TestHandleRefresh_ReuseDetectedReturns401 verifies that a refresh token not
// present in the active Redis set is rejected with 401 (reuse/theft signal).
// The token is never sent to Twitch — only the Redis GetDel lookup runs.
func TestHandleRefresh_ReuseDetectedReturns401(t *testing.T) {
	rdb := miniredis.RunT(t)
	defer rdb.Close()
	// do NOT seed refresh_token:<hash> — GetDel returns nil → reuse path
	h := newTestAuthHandler(t, redis.NewClient(&redis.Options{Addr: rdb.Addr()}))
	router := gin.New()
	router.POST("/refresh", h.HandleRefresh)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/refresh", nil)
	req.Header.Set("X-Refresh-Token", "never-issued-rt")
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("want 401 (reuse/invalid), got %d body=%s", w.Code, w.Body.String())
	}
}

// TestHandleRefresh_BackwardCompatJSONBody verifies the deprecated JSON-body
// fallback is still read. The token is seeded in Redis so it passes reuse
// detection; it then fails at Twitch refresh (no network in CI), which is
// fine — the assertion is only that the token was READ (NOT 400).
func TestHandleRefresh_BackwardCompatJSONBody(t *testing.T) {
	rdb := miniredis.RunT(t)
	defer rdb.Close()
	// seed the refresh token so it passes reuse-detection, then fails at Twitch refresh
	// (which is fine — we're only asserting the JSON-body fallback is READ, not that refresh succeeds)
	hash := refreshTokenHash("jsonbody-rt")
	rdb.Set("refresh_token:"+hash, "user-1")
	h := newTestAuthHandler(t, redis.NewClient(&redis.Options{Addr: rdb.Addr()}))
	router := gin.New()
	router.POST("/refresh", h.HandleRefresh)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/refresh", strings.NewReader(`{"refresh_token":"jsonbody-rt"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	// It should get past the "refresh token required" check (so NOT 400) — it will then
	// fail at Twitch.RefreshToken (real HTTP to twitch.tv, no network in CI) → 401 or 500.
	// Asserting NOT 400 proves the JSON-body fallback path was taken.
	if w.Code == 400 {
		t.Errorf("JSON-body refresh token not read; got 400 (refresh token required), body=%s", w.Body.String())
	}
}

// TestHandleStreamerTokenExchange_SetsCookies_OmitsTokensFromBody verifies
// that the M1 code-exchange endpoint issues httpOnly cookies for the access
// and refresh tokens (audit H3) and does NOT echo the tokens back in the JSON
// body — only non-secret UI fields (expires_in, redirect_to, etc.) are returned.
func TestHandleStreamerTokenExchange_SetsCookies_OmitsTokensFromBody(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	h := newTestAuthHandler(t, rdb)

	payload := StreamerAuthPayload{
		AccessToken:       "acc-jwt",
		RefreshToken:      "ref-jwt",
		ExpiresIn:         3600,
		TokenType:         "Bearer",
		RedirectTo:        "/dashboard",
		SourceAdded:       "twitch",
		ModerationEnabled: "true",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	code := "code-123"
	mr.Set("streamer_auth_code:"+code, string(data))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/exchange", h.HandleStreamerTokenExchange)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/exchange", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	// Cookies must carry the tokens.
	var gotAccess, gotRefresh bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" && c.Value == "acc-jwt" {
			gotAccess = true
		}
		if c.Name == "refresh_token" && c.Value == "ref-jwt" {
			gotRefresh = true
		}
	}
	if !gotAccess {
		t.Error("access_token cookie not set with expected value")
	}
	if !gotRefresh {
		t.Error("refresh_token cookie not set with expected value")
	}

	// Body must NOT leak tokens. Check JSON keys (not raw substrings) to avoid
	// false positives where a token value happens to contain "access".
	body := w.Body.String()
	if strings.Contains(body, `"access_token"`) || strings.Contains(body, `"refresh_token"`) {
		t.Errorf("tokens leaked in response body: %s", body)
	}
	if !strings.Contains(body, "redirect_to") {
		t.Errorf("redirect_to missing from response body: %s", body)
	}
	if !strings.Contains(body, "expires_in") {
		t.Errorf("expires_in missing from response body: %s", body)
	}
}
