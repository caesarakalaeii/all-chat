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

// The gateway's view of personal access tokens: the same middleware chain the real
// protected route group uses (CookieToBearer -> JWTAuthWithRevocation -> OriginCheck),
// exercised with a PAT instead of a session JWT.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/auth"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const patUserID = "pat-user-1"

// wireGatewayPATResolver installs a resolver that accepts exactly one token, for the
// duration of one test. The resolver is process-wide state (SetAPITokenResolver,
// mirroring SetLogger), so it is always restored.
func wireGatewayPATResolver(t *testing.T, token string, scopes []string) {
	t.Helper()
	want := string(sharedmiddleware.HashAPIToken(token))
	sharedmiddleware.SetAPITokenResolver(sharedmiddleware.APITokenResolverFunc(
		func(_ context.Context, hash []byte) (*sharedmiddleware.APITokenIdentity, error) {
			if string(hash) != want {
				return nil, sharedmiddleware.ErrAPITokenNotFound
			}
			return &sharedmiddleware.APITokenIdentity{
				TokenID: "tok-gw", UserID: patUserID, Username: "deckuser",
				Roles: []string{"user"}, Scopes: scopes,
			}, nil
		}))
	t.Cleanup(func() { sharedmiddleware.SetAPITokenResolver(nil) })
}

// gatewayChain mirrors the protectedAPI group in cmd/main.go.
func gatewayChain(t *testing.T, extra ...gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	kc := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-key-v1")},
		[]byte("test-secret-key"),
		"v1",
	)
	router := gin.New()
	router.Use(
		CookieToBearer(),
		sharedmiddleware.JWTAuthWithRevocation(kc, nil),
		sharedmiddleware.OriginCheck([]string{"https://allch.at"}),
	)
	handlers := append(extra, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})
	router.POST("/engagement/overlays/:id/polls", handlers...)
	return router
}

func TestGatewayAcceptsPersonalAccessToken(t *testing.T) {
	token := sharedmiddleware.APITokenPrefix + "gateway-happy-path"
	wireGatewayPATResolver(t, token, []string{sharedmiddleware.ScopeEngagementWrite})

	router := gatewayChain(t, sharedmiddleware.RequireAPITokenScope(sharedmiddleware.ScopeEngagementWrite))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engagement/overlays/ov-1/polls", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// A desktop plugin sends no Origin. OriginCheck deliberately allows that (it is a
	// CSRF defense for browsers), which is what makes a non-browser client workable.
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), patUserID)
}

func TestGatewayRejectsPATWithoutRequiredScope(t *testing.T) {
	token := sharedmiddleware.APITokenPrefix + "gateway-wrong-scope"
	wireGatewayPATResolver(t, token, []string{sharedmiddleware.ScopeChatWrite})

	router := gatewayChain(t, sharedmiddleware.RequireAPITokenScope(sharedmiddleware.ScopeEngagementWrite))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engagement/overlays/ov-1/polls", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	// It authenticated fine; it simply may not do this.
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestGatewayRejectsUnknownPAT(t *testing.T) {
	wireGatewayPATResolver(t, sharedmiddleware.APITokenPrefix+"issued", nil)

	router := gatewayChain(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engagement/overlays/ov-1/polls", nil)
	req.Header.Set("Authorization", "Bearer "+sharedmiddleware.APITokenPrefix+"never-issued")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}

// Regression guard: a session JWT through the very same chain, with a PAT resolver
// wired, must behave exactly as before — including on a scope-gated route, since a
// browser session is not scope-limited.
func TestGatewayJWTUnaffectedByPATPath(t *testing.T) {
	wireGatewayPATResolver(t, sharedmiddleware.APITokenPrefix+"unused", nil)

	jwtToken, err := auth.GenerateToken("user-123", "testuser", "test-secret-key", time.Hour, false)
	require.NoError(t, err)

	router := gatewayChain(t, sharedmiddleware.RequireAPITokenScope(sharedmiddleware.ScopeEngagementWrite))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engagement/overlays/ov-1/polls", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Origin", "https://allch.at")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "user-123")
}

// A PAT presented to a gateway that never called SetAPITokenResolver must be a clean
// 401 rather than a panic or an accidental JWT parse.
func TestGatewayWithoutResolverRejectsPAT(t *testing.T) {
	sharedmiddleware.SetAPITokenResolver(nil)

	router := gatewayChain(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engagement/overlays/ov-1/polls", nil)
	req.Header.Set("Authorization", "Bearer "+sharedmiddleware.APITokenPrefix+"anything")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
