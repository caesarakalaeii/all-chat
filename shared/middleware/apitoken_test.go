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

// Tests for the personal access token path. The resolver is process-wide state
// (SetAPITokenResolver, mirroring SetLogger), so every test that wires one restores
// the default with t.Cleanup and none of them run in parallel.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// fakeTokenRow is the subset of an api_tokens row the fake resolver needs to make the
// same accept/reject decisions the production SQL makes.
type fakeTokenRow struct {
	id        string
	userID    string
	username  string
	scopes    []string
	isAdmin   bool
	revoked   bool
	expiresAt *time.Time
}

// newFakeResolver builds an APITokenResolver over an in-memory table keyed by the
// plaintext token, applying the same validity rules as resolveAPITokenSQL: revoked or
// expired rows are indistinguishable from unknown ones.
func newFakeResolver(t *testing.T, rows map[string]fakeTokenRow) APITokenResolver {
	t.Helper()
	byHash := make(map[string]fakeTokenRow, len(rows))
	for plaintext, row := range rows {
		byHash[string(HashAPIToken(plaintext))] = row
	}
	return APITokenResolverFunc(func(_ context.Context, hash []byte) (*APITokenIdentity, error) {
		row, ok := byHash[string(hash)]
		if !ok || row.revoked {
			return nil, ErrAPITokenNotFound
		}
		if row.expiresAt != nil && !row.expiresAt.After(time.Now()) {
			return nil, ErrAPITokenNotFound
		}
		roles := []string{"user"}
		if row.isAdmin {
			roles = append(roles, "admin")
		}
		return &APITokenIdentity{
			TokenID:  row.id,
			UserID:   row.userID,
			Username: row.username,
			Roles:    roles,
			Scopes:   row.scopes,
		}, nil
	})
}

// wireResolver installs a resolver for the duration of one test.
func wireResolver(t *testing.T, r APITokenResolver) {
	t.Helper()
	SetAPITokenResolver(r)
	t.Cleanup(func() { SetAPITokenResolver(nil) })
}

// patRouter builds a router whose protected route echoes the resolved identity, so a
// test can assert the PAT path populated exactly what the JWT path would.
func patRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.GET("/protected", func(c *gin.Context) {
		roles, _ := c.Get("roles")
		c.JSON(http.StatusOK, gin.H{
			"user_id":     c.GetString("user_id"),
			"username":    c.GetString("username"),
			"roles":       roles,
			"auth_method": c.GetString(CtxAuthMethod),
			"token_id":    c.GetString(CtxAPITokenID),
		})
	})
	return router
}

func doGet(router *gin.Engine, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestAPIToken_ValidTokenAuthenticates(t *testing.T) {
	const token = APITokenPrefix + "valid-token-secret"
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-1", userID: "user-123", username: "streamer", scopes: []string{ScopeChatWrite}},
	}))

	resp := doGet(patRouter(), token)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for a valid PAT, got %d: %s", resp.Code, resp.Body.String())
	}

	var body struct {
		UserID     string   `json:"user_id"`
		Username   string   `json:"username"`
		Roles      []string `json:"roles"`
		AuthMethod string   `json:"auth_method"`
		TokenID    string   `json:"token_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// The whole point of the feature: identical context identity to a session JWT.
	if body.UserID != "user-123" || body.Username != "streamer" {
		t.Fatalf("PAT did not populate the JWT identity keys: %+v", body)
	}
	if len(body.Roles) != 1 || body.Roles[0] != "user" {
		t.Fatalf("expected roles [user], got %v", body.Roles)
	}
	if body.AuthMethod != AuthMethodAPIToken {
		t.Fatalf("expected auth_method %q, got %q", AuthMethodAPIToken, body.AuthMethod)
	}
	if body.TokenID != "tok-1" {
		t.Fatalf("expected token_id tok-1, got %q", body.TokenID)
	}
}

func TestAPIToken_RevokedTokenRejected(t *testing.T) {
	const token = APITokenPrefix + "revoked-token-secret"
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-2", userID: "user-123", username: "streamer", revoked: true},
	}))

	resp := doGet(patRouter(), token)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a revoked PAT, got %d", resp.Code)
	}
}

func TestAPIToken_ExpiredTokenRejected(t *testing.T) {
	const token = APITokenPrefix + "expired-token-secret"
	past := time.Now().Add(-time.Hour)
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-3", userID: "user-123", username: "streamer", expiresAt: &past},
	}))

	resp := doGet(patRouter(), token)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an expired PAT, got %d", resp.Code)
	}
}

func TestAPIToken_UnexpiredExpiryStillAuthenticates(t *testing.T) {
	const token = APITokenPrefix + "future-expiry-secret"
	future := time.Now().Add(time.Hour)
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-4", userID: "user-123", username: "streamer", expiresAt: &future},
	}))

	if resp := doGet(patRouter(), token); resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for a PAT with a future expiry, got %d", resp.Code)
	}
}

func TestAPIToken_UnknownTokenRejected(t *testing.T) {
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{}))

	if resp := doGet(patRouter(), APITokenPrefix+"never-issued"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unknown PAT, got %d", resp.Code)
	}
}

// A PAT presented to a service that never called SetAPITokenResolver must be a clean
// 401, not a panic and not an accidental JWT parse.
func TestAPIToken_NoResolverWiredRejects(t *testing.T) {
	SetAPITokenResolver(nil)

	if resp := doGet(patRouter(), APITokenPrefix+"whatever"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no resolver is wired, got %d", resp.Code)
	}
}

// Regression guard: the JWT path must be completely unchanged by the PAT branch,
// including when a resolver IS wired (a real service always has one).
func TestAPIToken_JWTStillAuthenticatesUnchanged(t *testing.T) {
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		APITokenPrefix + "some-pat": {id: "tok-5", userID: "pat-user", username: "pat"},
	}))

	token, err := auth.GenerateToken("user-999", "jwtuser", "test-secret", time.Hour, false)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	resp := doGet(patRouter(), token)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for a JWT, got %d: %s", resp.Code, resp.Body.String())
	}
	var body struct {
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
		AuthMethod string `json:"auth_method"`
		TokenID    string `json:"token_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.UserID != "user-999" || body.Username != "jwtuser" {
		t.Fatalf("JWT identity changed: %+v", body)
	}
	if body.AuthMethod != AuthMethodJWT {
		t.Fatalf("expected auth_method %q for a JWT, got %q", AuthMethodJWT, body.AuthMethod)
	}
	if body.TokenID != "" {
		t.Fatalf("JWT request must carry no api_token_id, got %q", body.TokenID)
	}
}

// scopeRouter mounts RequireAPITokenScope in front of a handler, in the same order a
// service does: authenticate, then scope, then the existing authorization gates.
func scopeRouter(required ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.POST("/polls", RequireAPITokenScope(required...), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func doPost(router *gin.Engine, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/polls", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestAPIToken_ScopeGrantsAccess(t *testing.T) {
	const token = APITokenPrefix + "scoped-ok"
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-6", userID: "user-123", username: "streamer",
			scopes: []string{ScopeChatWrite, ScopeEngagementWrite}},
	}))

	if resp := doPost(scopeRouter(ScopeEngagementWrite), token); resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for a PAT holding the scope, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAPIToken_MissingScopeRejected(t *testing.T) {
	const token = APITokenPrefix + "scoped-missing"
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-7", userID: "user-123", username: "streamer",
			scopes: []string{ScopeChatWrite}},
	}))

	resp := doPost(scopeRouter(ScopeEngagementWrite), token)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a PAT lacking the scope, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), ScopeEngagementWrite) {
		t.Fatalf("403 body should name the required scope, got %s", resp.Body.String())
	}
}

func TestAPIToken_EmptyScopesRejected(t *testing.T) {
	const token = APITokenPrefix + "scoped-none"
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{
		token: {id: "tok-8", userID: "user-123", username: "streamer"},
	}))

	if resp := doPost(scopeRouter(ScopeEngagementWrite), token); resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a PAT with no scopes, got %d", resp.Code)
	}
}

// A session JWT is not scope-limited: RequireAPITokenScope must be a no-op for it,
// otherwise mounting it on an existing route would break every browser client.
func TestAPIToken_ScopeMiddlewareIgnoresJWT(t *testing.T) {
	wireResolver(t, newFakeResolver(t, map[string]fakeTokenRow{}))

	token, err := auth.GenerateToken("user-999", "jwtuser", "test-secret", time.Hour, false)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if resp := doPost(scopeRouter(ScopeEngagementWrite), token); resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for a session JWT on a scoped route, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGenerateAPIToken(t *testing.T) {
	plaintext, hash, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}
	if !IsAPIToken(plaintext) {
		t.Fatalf("generated token %q lacks the %q prefix", plaintext, APITokenPrefix)
	}
	secret := strings.TrimPrefix(plaintext, APITokenPrefix)
	if len(secret) < 40 {
		t.Fatalf("generated secret is too short (%d chars): %q", len(secret), secret)
	}
	if !bytes.Equal(hash, HashAPIToken(plaintext)) {
		t.Fatalf("returned hash does not match HashAPIToken(plaintext)")
	}
	if len(hash) != 32 {
		t.Fatalf("expected a 32-byte SHA-256 digest, got %d bytes", len(hash))
	}

	// Two mints must never collide.
	other, otherHash, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken (second): %v", err)
	}
	if other == plaintext || bytes.Equal(hash, otherHash) {
		t.Fatalf("two generated tokens collided")
	}
}

func TestIsAPIToken(t *testing.T) {
	cases := map[string]bool{
		APITokenPrefix + "abc":     true,
		APITokenPrefix:             true,
		"allchat_pat":              false,
		"eyJhbGciOiJIUzI1NiJ9.a.b": false,
		"":                         false,
		"Xallchat_pat_abc":         false,
		"ALLCHAT_PAT_abc":          false,
	}
	for value, want := range cases {
		if got := IsAPIToken(value); got != want {
			t.Errorf("IsAPIToken(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestAPITokenScopes_NotAPIToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if scopes, ok := APITokenScopes(c); ok || scopes != nil {
		t.Fatalf("expected (nil,false) for an unauthenticated context, got (%v,%v)", scopes, ok)
	}
	c.Set(CtxAuthMethod, AuthMethodJWT)
	if _, ok := APITokenScopes(c); ok {
		t.Fatalf("expected ok=false for a JWT-authenticated context")
	}
}
