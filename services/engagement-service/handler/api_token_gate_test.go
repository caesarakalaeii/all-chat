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

package handler

// Pins the engagement route contract for personal access tokens (Stream Deck /
// StreamController plugins), in the exact middleware order main.go uses:
//
//	JWTAuthWithRevocation -> RequireAPITokenScope -> RequirePremium -> handler
//
// The load-bearing assertion is the last one: a PAT is subject to the SAME premium
// gate as a browser session. Scopes narrow a token; they never bypass a gate, and this
// file fails if someone ever "fixes" a plugin 403 by dropping RequirePremium.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/featuregates"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	patPollUserID = "11111111-1111-1111-1111-111111111111"
	testJWTSecret = "engagement-test-secret"
)

// keyChain matches what main.go builds from JWT_SECRET_V1.
func keyChain() *sharedAuth.KeyChain {
	return sharedAuth.NewKeyChain(
		map[string][]byte{"v1": []byte(testJWTSecret + "-v1")},
		[]byte(testJWTSecret),
		"v1",
	)
}

// wirePAT installs a resolver that accepts one token with the given scopes, restoring
// the package default afterwards (the resolver is process-wide, like SetLogger).
func wirePAT(t *testing.T, token string, scopes []string) {
	t.Helper()
	want := string(middleware.HashAPIToken(token))
	middleware.SetAPITokenResolver(middleware.APITokenResolverFunc(
		func(_ context.Context, hash []byte) (*middleware.APITokenIdentity, error) {
			if string(hash) != want {
				return nil, middleware.ErrAPITokenNotFound
			}
			return &middleware.APITokenIdentity{
				TokenID: "tok-eng", UserID: patPollUserID, Username: "deckuser",
				Roles: []string{"user"}, Scopes: scopes,
			}, nil
		}))
	t.Cleanup(func() { middleware.SetAPITokenResolver(nil) })
}

// createPollRouter mirrors main.go's POST /overlays/:id/polls chain, including the
// premium gate, with an injectable premium answer.
func createPollRouter(userPremium bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	gate := featuregates.NewFeatureGateCacheWithGates(map[string]bool{featuregates.GateEngagement: true})
	querier := func(context.Context, string) (bool, error) { return userPremium, nil }

	r := gin.New()
	auth := r.Group("/")
	auth.Use(middleware.JWTAuthWithRevocation(keyChain(), nil))
	auth.POST("/overlays/:id/polls",
		middleware.RequireAPITokenScope(middleware.ScopeEngagementWrite),
		middleware.RequirePremiumWithQuerier(gate, featuregates.GateEngagement, querier, nil),
		func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"user_id": c.GetString("user_id")})
		})
	// close/lock/resolve/cancel are deliberately NOT premium-gated (managing an
	// already-open round must keep working for a non-premium owner), but they DO carry
	// the scope check.
	auth.POST("/overlays/:id/polls/:pollId/close",
		middleware.RequireAPITokenScope(middleware.ScopeEngagementWrite),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func post(r *gin.Engine, path, bearer string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	r.ServeHTTP(w, req)
	return w
}

// A premium owner's PAT with engagement:write may start a round — the whole point of
// the feature.
func TestPAT_PremiumOwnerCanStartARound(t *testing.T) {
	token := middleware.APITokenPrefix + "engagement-premium"
	wirePAT(t, token, []string{middleware.ScopeEngagementWrite})

	w := post(createPollRouter(true), "/overlays/ov-1/polls", token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), patPollUserID, "the PAT must land the same user_id a JWT would")
}

// THE gate assertion: a PAT is authentication, never an authorization bypass. A
// non-premium owner's perfectly valid, correctly scoped token is still 403'd by
// RequirePremium, exactly as their browser session is.
func TestPAT_DoesNotBypassThePremiumGate(t *testing.T) {
	token := middleware.APITokenPrefix + "engagement-nonpremium"
	wirePAT(t, token, []string{middleware.ScopeEngagementWrite})

	w := post(createPollRouter(false), "/overlays/ov-1/polls", token)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Premium feature required",
		"a scoped PAT must still be refused by the premium gate, not by the scope check")
}

// The scope check runs BEFORE the premium query, so an unscoped token never reaches it.
func TestPAT_MissingScopeIsRejected(t *testing.T) {
	token := middleware.APITokenPrefix + "engagement-chat-only"
	wirePAT(t, token, []string{middleware.ScopeChatWrite})

	w := post(createPollRouter(true), "/overlays/ov-1/polls", token)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "insufficient token scope")
}

func TestPAT_UnknownTokenIsRejected(t *testing.T) {
	wirePAT(t, middleware.APITokenPrefix+"issued", []string{middleware.ScopeEngagementWrite})

	w := post(createPollRouter(true), "/overlays/ov-1/polls", middleware.APITokenPrefix+"revoked-or-expired")
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}

// Managing an already-open round stays ungated on premium (only scoped), so a plugin
// owned by a lapsed subscriber can still close the poll it opened.
func TestPAT_ClosingARoundIsNotPremiumGated(t *testing.T) {
	token := middleware.APITokenPrefix + "engagement-close"
	wirePAT(t, token, []string{middleware.ScopeEngagementWrite})

	w := post(createPollRouter(false), "/overlays/ov-1/polls/p-1/close", token)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// Regression guard: a session JWT through the identical chain is unaffected — it
// authenticates as before and the scope middleware is a no-op for it, so the premium
// gate remains the only thing that can refuse it.
func TestPAT_SessionJWTUnaffected(t *testing.T) {
	wirePAT(t, middleware.APITokenPrefix+"unused", nil)

	jwt, err := sharedAuth.GenerateToken(patPollUserID, "webuser", testJWTSecret, time.Hour, false)
	require.NoError(t, err)

	w := post(createPollRouter(true), "/overlays/ov-1/polls", jwt)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = post(createPollRouter(false), "/overlays/ov-1/polls", jwt)
	assert.Equal(t, http.StatusForbidden, w.Code, "a non-premium session is still gated, unchanged")
}
