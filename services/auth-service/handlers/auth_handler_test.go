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

// TestHandleLogout_ClearsCookiesAndRevokesRefresh verifies that /logout
// blacklists the access JWT, revokes the refresh-token Redis key, and clears
// the auth cookies (audit H3). It exercises the X-Access-Token / X-Refresh-Token
// header path that the gateway AuthCookieForward middleware forwards from the
// httpOnly cookies (raw Cookie stripped by L17 origin_check).
func TestHandleLogout_ClearsCookiesAndRevokesRefresh(t *testing.T) {
	rdb := miniredis.RunT(t)
	defer rdb.Close()
	// pre-seed a refresh_token:<hash>
	hash := refreshTokenHash("some-rt")
	rdb.Set("refresh_token:"+hash, "user-1")

	h := newTestAuthHandler(t, redis.NewClient(&redis.Options{Addr: rdb.Addr()}))
	router := gin.New()
	router.POST("/logout", h.HandleLogout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", nil)
	req.Header.Set("X-Access-Token", "jwt-to-blacklist")
	req.Header.Set("X-Refresh-Token", "some-rt")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	// access token blacklisted
	if !rdb.Exists("blacklist:jwt-to-blacklist") {
		t.Error("access token not blacklisted")
	}
	// refresh token Redis key deleted
	if rdb.Exists("refresh_token:" + hash) {
		t.Error("refresh token not revoked")
	}

	// cookies cleared
	var cleared int
	for _, c := range w.Result().Cookies() {
		if c.Value == "" || c.MaxAge == -1 {
			cleared++
		}
	}
	if cleared < 2 {
		t.Errorf("want 2 cleared cookies, got %d", cleared)
	}
}

// TestHandleStreamerTokenExchange_SeedsRefreshTokenReuseKey (audit C2)
// verifies that /exchange seeds the refresh_token:<hash> reuse-detection key
// when issuing cookies. Without this, HandleRefresh's GetDel treats the FIRST
// refresh after a real login as token theft → 401 + ClearAuthCookies, force-
// logging out every user. The handler must Set the key so the first /refresh
// succeeds.
func TestHandleStreamerTokenExchange_SeedsRefreshTokenReuseKey(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	h := newTestAuthHandler(t, rdb)

	rt := "exchange-issued-refresh-token"
	code := "code-c2"
	payload := StreamerAuthPayload{
		AccessToken:  "acc-jwt",
		RefreshToken: rt,
		ExpiresIn:    3600,
		TokenType:    "Bearer",
		User:         &StreamerAuthUser{ID: "user-c2", Username: "u"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	mr.Set("streamer_auth_code:"+code, string(data))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/exchange", h.HandleStreamerTokenExchange)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/exchange", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("exchange status=%d body=%s", w.Code, w.Body.String())
	}

	// C2 invariant: the reuse key MUST exist after exchange so the first /refresh
	// isn't misclassified as theft.
	rtKey := "refresh_token:" + refreshTokenHash(rt)
	if !mr.Exists(rtKey) {
		t.Fatalf("C2 regression: refresh-token reuse key not seeded by /exchange; " +
			"first /refresh after login would force-logout the user")
	}
}

// TestHandleRefresh_AfterExchange_DoesNotForceLogout (audit C2 + C5) verifies
// the end-to-end contract that C1+C2 restore: after /exchange seeds the reuse
// key, a /refresh with that token passes the GetDel reuse check (does NOT 401).
// It stops short of calling Twitch (no network in CI) — the assertion is that
// the reuse-detection gate is OPEN (not 401-reuse), proving the first refresh
// after login is no longer misread as theft. Pre-C2 this returned 401
// "Refresh token already used or invalid".
func TestHandleRefresh_AfterExchange_DoesNotForceLogout(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	h := newTestAuthHandler(t, rdb)

	rt := "exchange-issued-refresh-token"
	// Simulate the C2 seeding that /exchange now performs.
	rtKey := "refresh_token:" + refreshTokenHash(rt)
	mr.Set(rtKey, "user-c2")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/refresh", h.HandleRefresh)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/refresh", nil)
	req.Header.Set("X-Refresh-Token", rt)
	router.ServeHTTP(w, req)

	// The reuse check passed iff we did NOT get the reuse 401. The request then
	// proceeds to Twitch.RefreshToken (real HTTP, no network in CI) → some
	// non-401 error (503 transient or 401 terminal). Either way it must NOT be
	// the "Refresh token already used or invalid" reuse 401.
	if w.Code == 401 {
		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["error"] == "Refresh token already used or invalid" {
			t.Fatalf("C2 regression: first /refresh after exchange was misread as "+
				"reuse (401) even though /exchange seeded the key; body=%s", w.Body.String())
		}
	}
	// Reuse key was consumed by GetDel (proving the gate ran and passed).
	if mr.Exists(rtKey) {
		t.Fatalf("reuse key not consumed by /refresh GetDel")
	}
}

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
		User: &StreamerAuthUser{
			ID:       "user-1",
			Username: "u",
			IsAdmin:  false,
		},
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
	if !strings.Contains(body, "user") {
		t.Errorf("user missing from response body: %s", body)
	}
	if !strings.Contains(body, "user-1") {
		t.Errorf("user id missing from response body: %s", body)
	}
}
