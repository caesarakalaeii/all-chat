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

// Tests for the paired-device token path (ADR-0049). The resolver is process-wide
// state, so every test that wires one restores the default with t.Cleanup and none of
// them run in parallel — same discipline as apitoken_test.go.
//
// Two assertions here are the load-bearing ones:
//
//   - IsAPIToken must accept `allchat_dev_`. If it does not, a device token falls
//     through to the JWT parser and 401s in every service; nothing else in the feature
//     works.
//   - RequireDeviceTokenOverlay must 403 a device token on an overlay it was not paired
//     with, and must NOT touch a session or a PAT.

import (
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

// fakeDeviceRow is the subset of a device_tokens row the fake resolver needs to make
// the same accept/reject decisions resolveDeviceTokenSQL makes.
type fakeDeviceRow struct {
	id        string
	userID    string
	username  string
	overlayID string
	scopes    []string
	revoked   bool
	expiresAt time.Time
	banned    bool
}

// newFakeDeviceResolver builds a resolver over an in-memory device_tokens, applying the
// production predicates: revoked, expired and banned-owner rows are indistinguishable
// from unknown ones.
func newFakeDeviceResolver(t *testing.T, rows map[string]fakeDeviceRow) APITokenResolver {
	t.Helper()
	byHash := make(map[string]fakeDeviceRow, len(rows))
	for plaintext, row := range rows {
		byHash[string(HashDeviceToken(plaintext))] = row
	}
	return APITokenResolverFunc(func(_ context.Context, hash []byte) (*APITokenIdentity, error) {
		row, ok := byHash[string(hash)]
		if !ok || row.revoked || row.banned {
			return nil, ErrAPITokenNotFound
		}
		// expires_at is NOT NULL for a device token, so an unset value is expired.
		if !row.expiresAt.After(time.Now()) {
			return nil, ErrAPITokenNotFound
		}
		return &APITokenIdentity{
			TokenID:   row.id,
			UserID:    row.userID,
			Username:  row.username,
			Roles:     []string{"user"},
			Scopes:    row.scopes,
			Kind:      TokenKindDevice,
			OverlayID: row.overlayID,
		}, nil
	})
}

// liveDeviceRow is a valid row: not revoked, well within its sliding window.
func liveDeviceRow(id, userID, overlayID string, scopes ...string) fakeDeviceRow {
	return fakeDeviceRow{
		id:        id,
		userID:    userID,
		username:  "streamer",
		overlayID: overlayID,
		scopes:    scopes,
		expiresAt: time.Now().Add(DeviceTokenLifetime),
	}
}

// deviceRouter builds a router shaped like the engagement routes: JWT/token auth, then
// the additive scope and overlay-binding middleware, then a handler that echoes what
// the auth path resolved.
func deviceRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1")},
		[]byte("test-secret"),
		"v1",
	)
	router := gin.New()
	router.Use(JWTAuth(kc))
	router.POST("/overlays/:id/polls",
		RequireAPITokenScope(ScopeEngagementWrite),
		RequireDeviceTokenOverlay("id"),
		func(c *gin.Context) {
			overlay, isDevice := DeviceTokenOverlay(c)
			c.JSON(http.StatusOK, gin.H{
				"user_id":     c.GetString("user_id"),
				"auth_method": c.GetString(CtxAuthMethod),
				"token_kind":  c.GetString(CtxTokenKind),
				"bound":       overlay,
				"is_device":   isDevice,
			})
		})
	return router
}

func doPollPost(router *gin.Engine, overlayID, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/overlays/"+overlayID+"/polls", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestIsAPIToken_RecognisesDevicePrefix(t *testing.T) {
	// THE assertion this whole feature rests on. IsAPIToken is what routes a bearer
	// away from the JWT parser (shared/middleware/auth.go), away from the logout
	// blacklist (whose key embeds the plaintext) and away from CookieToBearer. A device
	// token that failed this test would 401 in every service with no useful error.
	cases := map[string]bool{
		DeviceTokenPrefix + "abc":  true,
		DeviceTokenPrefix:          true,
		APITokenPrefix + "abc":     true,
		"allchat_dev":              false,
		"ALLCHAT_DEV_abc":          false,
		"Xallchat_dev_abc":         false,
		"eyJhbGciOiJIUzI1NiJ9.a.b": false,
		"":                         false,
	}
	for value, want := range cases {
		if got := IsAPIToken(value); got != want {
			t.Errorf("IsAPIToken(%q) = %v, want %v — a device token that is not "+
				"recognised here is parsed as a JWT and 401s everywhere", value, got, want)
		}
	}
}

func TestIsDeviceToken_DistinguishesFromPAT(t *testing.T) {
	if !IsDeviceToken(DeviceTokenPrefix + "abc") {
		t.Error("IsDeviceToken rejected a device token")
	}
	if IsDeviceToken(APITokenPrefix + "abc") {
		t.Error("IsDeviceToken accepted a PAT; the two credential types must stay distinguishable")
	}
}

func TestGenerateDeviceToken(t *testing.T) {
	plaintext, hash, err := GenerateDeviceToken()
	if err != nil {
		t.Fatalf("GenerateDeviceToken: %v", err)
	}
	if !strings.HasPrefix(plaintext, DeviceTokenPrefix) {
		t.Errorf("token %q lacks the %q prefix", plaintext, DeviceTokenPrefix)
	}
	if IsDeviceToken(plaintext) != true || !IsAPIToken(plaintext) {
		t.Error("a generated device token must satisfy both IsDeviceToken and IsAPIToken")
	}
	// 256 bits base64url-unpadded is 43 characters, the same shape GenerateAPIToken
	// produces. A shorter secret would be a silently weakened credential.
	if got := len(strings.TrimPrefix(plaintext, DeviceTokenPrefix)); got != 43 {
		t.Errorf("secret length = %d, want 43 (256 bits, base64url unpadded)", got)
	}
	if len(hash) != 32 {
		t.Errorf("digest length = %d, want 32 (SHA-256)", len(hash))
	}
	if strings.Contains(string(hash), plaintext) {
		t.Error("the digest must not contain the plaintext")
	}
	second, _, err := GenerateDeviceToken()
	if err != nil {
		t.Fatalf("GenerateDeviceToken (second): %v", err)
	}
	if second == plaintext {
		t.Fatal("two generated device tokens collided")
	}
}

func TestDeviceToken_AuthenticatesAndBinds(t *testing.T) {
	const token = DeviceTokenPrefix + "device-secret"
	const overlay = "11111111-1111-1111-1111-111111111111"
	wireResolver(t, newFakeDeviceResolver(t, map[string]fakeDeviceRow{
		token: liveDeviceRow("dev-1", "user-9", overlay, ScopeEngagementWrite),
	}))

	resp := doPollPost(deviceRouter(), overlay, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.Code, resp.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["user_id"] != "user-9" {
		t.Errorf("user_id = %v, want user-9 (a device token must populate the same identity a JWT does)", body["user_id"])
	}
	if body["auth_method"] != AuthMethodAPIToken {
		t.Errorf("auth_method = %v, want %q", body["auth_method"], AuthMethodAPIToken)
	}
	if body["token_kind"] != TokenKindDevice {
		t.Errorf("token_kind = %v, want %q", body["token_kind"], TokenKindDevice)
	}
	if body["bound"] != overlay {
		t.Errorf("bound overlay = %v, want %v", body["bound"], overlay)
	}
}

func TestDeviceToken_WrongOverlayRefused(t *testing.T) {
	const token = DeviceTokenPrefix + "device-secret"
	const paired = "11111111-1111-1111-1111-111111111111"
	const other = "22222222-2222-2222-2222-222222222222"
	wireResolver(t, newFakeDeviceResolver(t, map[string]fakeDeviceRow{
		token: liveDeviceRow("dev-1", "user-9", paired, ScopeEngagementWrite),
	}))

	resp := doPollPost(deviceRouter(), other, token)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403. The per-overlay binding is the property a PAT "+
			"structurally cannot have (ADR-0049): a compromised control surface must not be "+
			"able to drive a different overlay. Body = %s", resp.Code, resp.Body.String())
	}
}

func TestDeviceToken_RevokedExpiredAndBannedAllRejected(t *testing.T) {
	const overlay = "11111111-1111-1111-1111-111111111111"
	const revoked = DeviceTokenPrefix + "revoked"
	const expired = DeviceTokenPrefix + "expired"
	const banned = DeviceTokenPrefix + "banned"

	revokedRow := liveDeviceRow("dev-r", "user-9", overlay, ScopeEngagementWrite)
	revokedRow.revoked = true
	expiredRow := liveDeviceRow("dev-e", "user-9", overlay, ScopeEngagementWrite)
	expiredRow.expiresAt = time.Now().Add(-time.Minute)
	bannedRow := liveDeviceRow("dev-b", "user-9", overlay, ScopeEngagementWrite)
	bannedRow.banned = true

	wireResolver(t, newFakeDeviceResolver(t, map[string]fakeDeviceRow{
		revoked: revokedRow,
		expired: expiredRow,
		banned:  bannedRow,
	}))

	router := deviceRouter()
	for name, token := range map[string]string{
		"revoked":      revoked,
		"expired":      expired,
		"banned owner": banned,
	} {
		resp := doPollPost(router, overlay, token)
		if resp.Code != http.StatusUnauthorized {
			t.Errorf("%s device token: status = %d, want 401", name, resp.Code)
		}
	}
}

func TestDeviceToken_MissingScopeRefused(t *testing.T) {
	// Scopes still bound a device token: the overlay binding and the scope set are
	// independent narrowings, and neither replaces the other.
	const token = DeviceTokenPrefix + "chat-only"
	const overlay = "11111111-1111-1111-1111-111111111111"
	wireResolver(t, newFakeDeviceResolver(t, map[string]fakeDeviceRow{
		token: liveDeviceRow("dev-1", "user-9", overlay, ScopeChatWrite),
	}))

	resp := doPollPost(deviceRouter(), overlay, token)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a device token without engagement:write", resp.Code)
	}
}

func TestRequireDeviceTokenOverlay_IgnoresSessionAndPAT(t *testing.T) {
	// A session is not overlay-limited (a streamer may drive any overlay they own) and a
	// PAT is user-scoped by construction, so mounting this middleware on a route must
	// not change behaviour for either. If this test fails, wiring the binding onto the
	// engagement routes has broken the dashboard.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/overlays/:id", RequireDeviceTokenOverlay("id"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Unauthenticated context: nothing set at all, as a public route would be.
	req := httptest.NewRequest(http.MethodGet, "/overlays/any-overlay", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("session/anonymous status = %d, want 200", resp.Code)
	}

	// PAT context: auth_method is api_token but the kind is pat, so no binding exists.
	patRouterWithBinding := gin.New()
	patRouterWithBinding.GET("/overlays/:id", func(c *gin.Context) {
		c.Set(CtxAuthMethod, AuthMethodAPIToken)
		c.Set(CtxTokenKind, TokenKindPAT)
		c.Next()
	}, RequireDeviceTokenOverlay("id"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req = httptest.NewRequest(http.MethodGet, "/overlays/any-overlay", nil)
	resp = httptest.NewRecorder()
	patRouterWithBinding.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PAT status = %d, want 200 (a PAT is user-scoped and has no overlay binding)", resp.Code)
	}
}

func TestTokenResolverDispatch_RoutesBothCredentialTypes(t *testing.T) {
	const pat = APITokenPrefix + "pat-secret"
	const device = DeviceTokenPrefix + "device-secret"
	const overlay = "11111111-1111-1111-1111-111111111111"

	patResolver := newFakeResolver(t, map[string]fakeTokenRow{
		pat: {id: "pat-1", userID: "user-1", username: "streamer", scopes: []string{ScopeChatWrite}},
	})
	deviceResolver := newFakeDeviceResolver(t, map[string]fakeDeviceRow{
		device: liveDeviceRow("dev-1", "user-2", overlay, ScopeChatWrite),
	})
	dispatch := NewTokenResolverDispatch(patResolver, deviceResolver)

	got, err := dispatch.ResolveAPIToken(context.Background(), HashAPIToken(pat))
	if err != nil {
		t.Fatalf("PAT through dispatcher: %v", err)
	}
	if got.UserID != "user-1" || got.Kind == TokenKindDevice {
		t.Errorf("PAT resolved as %+v, want the api_tokens row shape", got)
	}

	got, err = dispatch.ResolveAPIToken(context.Background(), HashDeviceToken(device))
	if err != nil {
		t.Fatalf("device token through dispatcher: %v", err)
	}
	if got.UserID != "user-2" || got.Kind != TokenKindDevice || got.OverlayID != overlay {
		t.Errorf("device token resolved as %+v, want the device_tokens row shape", got)
	}

	if _, err := dispatch.ResolveAPIToken(context.Background(), HashAPIToken("unknown")); err == nil {
		t.Error("dispatcher accepted an unknown digest")
	}
}

func TestTokenResolverDispatch_PropagatesRealErrors(t *testing.T) {
	// A database outage must not be reported as ErrAPITokenNotFound: that would turn a
	// transient failure into an oracle telling a client the credential does not exist,
	// and would also hide the outage from operators (authenticateAPIToken only logs
	// non-NotFound errors).
	boom := APITokenResolverFunc(func(context.Context, []byte) (*APITokenIdentity, error) {
		return nil, context.DeadlineExceeded
	})
	dispatch := NewTokenResolverDispatch(nil, boom)
	if _, err := dispatch.ResolveAPIToken(context.Background(), []byte("x")); err == nil {
		t.Fatal("dispatcher swallowed a resolver failure")
	}
}
