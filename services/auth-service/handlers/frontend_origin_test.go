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
	"net/url"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func originTestContext(host, forwardedHost string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "https://"+host+"/twitch/login", nil)
	if forwardedHost != "" {
		req.Header.Set("X-Forwarded-Host", forwardedHost)
	}
	c.Request = req
	return c
}

func TestRequestFrontendOrigin(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allch.at")
	t.Setenv("FRONTEND_URLS", "https://beta.allch.at")

	tests := []struct {
		name          string
		host          string
		forwardedHost string
		want          string
	}{
		{"allowlisted beta host via gateway header", "auth-service:8081", "beta.allch.at", "https://beta.allch.at"},
		{"allowlisted prod host via gateway header", "auth-service:8081", "allch.at", "https://allch.at"},
		{"unknown forwarded host falls back to canonical", "auth-service:8081", "evil.example.com", "https://allch.at"},
		{"first forwarded entry wins", "auth-service:8081", "beta.allch.at, evil.example.com", "https://beta.allch.at"},
		{"no gateway header uses request host", "allch.at", "", "https://allch.at"},
		{"trailing slash in FRONTEND_URLS still matches", "auth-service:8081", "beta.allch.at", "https://beta.allch.at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, requestFrontendOrigin(originTestContext(tt.host, tt.forwardedHost)))
		})
	}
}

func TestRequestFrontendOrigin_NoAllowlistConfigured(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allch.at")
	t.Setenv("FRONTEND_URLS", "")

	// Without FRONTEND_URLS the beta host is not first-party and the flow must
	// return to the canonical origin, never to the unverified request host.
	c := originTestContext("auth-service:8081", "beta.allch.at")
	assert.Equal(t, "https://allch.at", requestFrontendOrigin(c))
}

func TestStateOrigin(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allch.at")
	t.Setenv("FRONTEND_URLS", "https://beta.allch.at")

	t.Run("nil state falls back to canonical", func(t *testing.T) {
		assert.Equal(t, "https://allch.at", stateOrigin(nil))
	})
	t.Run("empty origin is canonical (states predating the field)", func(t *testing.T) {
		assert.Equal(t, "https://allch.at", stateOrigin(&oauth.OAuthState{}))
	})
	t.Run("allowlisted origin survives", func(t *testing.T) {
		s := &oauth.OAuthState{Origin: "https://beta.allch.at"}
		assert.Equal(t, "https://beta.allch.at", stateOrigin(s))
	})
	t.Run("non-allowlisted origin is rejected", func(t *testing.T) {
		s := &oauth.OAuthState{Origin: "https://evil.example.com"}
		assert.Equal(t, "https://allch.at", stateOrigin(s))
	})
}

func TestProviderForOrigin(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allch.at")
	t.Setenv("FRONTEND_URLS", "https://beta.allch.at")

	canonical := "https://allch.at/api/v1/auth/twitch/callback"
	twitch := oauth.NewTwitchOAuth("id", "secret", canonical)

	t.Run("canonical origin returns the provider unchanged", func(t *testing.T) {
		assert.Same(t, twitch, providerForOrigin(twitch, oauth.PlatformTwitch, "https://allch.at"))
	})
	t.Run("beta origin swaps the redirect URI", func(t *testing.T) {
		p := providerForOrigin(twitch, oauth.PlatformTwitch, "https://beta.allch.at")
		authURL, err := url.Parse(p.GetAuthURL("state"))
		require.NoError(t, err)
		assert.Equal(t,
			"https://beta.allch.at/api/v1/auth/twitch/callback",
			authURL.Query().Get("redirect_uri"))
		// The original provider is untouched.
		origURL, _ := url.Parse(twitch.GetAuthURL("state"))
		assert.Equal(t, canonical, origURL.Query().Get("redirect_uri"))
	})
}

// A login started on the beta host must carry that origin into the provider's
// redirect_uri and into the server-side state the callback later reads.
func TestHandleLogin_RecordsAllowlistedOrigin(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allch.at")
	t.Setenv("FRONTEND_URLS", "https://beta.allch.at")

	mr := miniredis.RunT(t)
	h := &PlatformAuthHandlerV2{
		providers: map[oauth.Platform]oauth.OAuthProvider{
			oauth.PlatformTwitch: oauth.NewTwitchOAuth("id", "secret", "https://allch.at/api/v1/auth/twitch/callback"),
		},
		redis:       redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		logger:      zap.NewNop(),
		frontendURL: "https://allch.at",
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/twitch/login", h.HandleLogin(oauth.PlatformTwitch))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://auth-service:8081/twitch/login", nil)
	req.Header.Set("X-Forwarded-Host", "beta.allch.at")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	authURL, err := url.Parse(body["auth_url"])
	require.NoError(t, err)
	assert.Equal(t,
		"https://beta.allch.at/api/v1/auth/twitch/callback",
		authURL.Query().Get("redirect_uri"))

	// Twitch login also stashes the PKCE verifier; the state is the other key.
	var stateKeys []string
	for _, k := range mr.Keys() {
		if strings.HasPrefix(k, "oauth_state:") {
			stateKeys = append(stateKeys, k)
		}
	}
	require.Len(t, stateKeys, 1, "expected exactly the stored oauth state")
	stored, err := mr.Get(stateKeys[0])
	require.NoError(t, err)
	var state oauth.OAuthState
	require.NoError(t, json.Unmarshal([]byte(stored), &state))
	assert.Equal(t, "https://beta.allch.at", state.Origin)
}

// Without the allowlist header the same handler must keep redirecting to the
// canonical callback exactly as before the origin plumbing existed.
func TestHandleLogin_UnknownHostStaysCanonical(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allch.at")
	t.Setenv("FRONTEND_URLS", "https://beta.allch.at")

	mr := miniredis.RunT(t)
	h := &PlatformAuthHandlerV2{
		providers: map[oauth.Platform]oauth.OAuthProvider{
			oauth.PlatformTwitch: oauth.NewTwitchOAuth("id", "secret", "https://allch.at/api/v1/auth/twitch/callback"),
		},
		redis:       redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		logger:      zap.NewNop(),
		frontendURL: "https://allch.at",
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/twitch/login", h.HandleLogin(oauth.PlatformTwitch))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://auth-service:8081/twitch/login", nil)
	req.Header.Set("X-Forwarded-Host", "evil.example.com")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	authURL, err := url.Parse(body["auth_url"])
	require.NoError(t, err)
	assert.Equal(t,
		"https://allch.at/api/v1/auth/twitch/callback",
		authURL.Query().Get("redirect_uri"))
}
