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
	"context"
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

// modConsentTestHandler builds the minimum PlatformAuthHandlerV2 the start endpoint needs: real
// OAuth providers (they only build URLs) and a real-enough Redis for the state stash.
func modConsentTestHandler(t *testing.T) *PlatformAuthHandlerV2 {
	t.Helper()
	mr := miniredis.RunT(t)
	return &PlatformAuthHandlerV2{
		providers: map[oauth.Platform]oauth.OAuthProvider{
			oauth.PlatformTwitch: oauth.NewTwitchOAuth("client-id", "client-secret", "https://allch.at/cb"),
			oauth.PlatformKick:   oauth.NewKickOAuth("kick-id", "kick-secret", "https://allch.at/cb"),
		},
		redis:       redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		logger:      zap.NewNop(),
		frontendURL: "https://allch.at",
	}
}

func startModConsent(t *testing.T, h *PlatformAuthHandlerV2, platform oauth.Platform, userID, actions string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/:platform/mod-consent", func(c *gin.Context) {
		if userID != "" {
			c.Set("user_id", userID)
		}
		h.HandleModConsent(platform)(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/"+string(platform)+"/mod-consent?actions="+actions, nil)
	r.ServeHTTP(w, req)
	return w
}

func authURLFrom(t *testing.T, w *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body["auth_url"], "response carried no auth_url: %s", w.Body.String())
	parsed, err := url.Parse(body["auth_url"])
	require.NoError(t, err)
	return parsed
}

// The consent screen must carry exactly the scopes for the delegated actions — no base login
// scopes, and no chat-send scope. Moderators get no send capability in v1, and the streamer flow
// deliberately folds send in, so this is the one place that difference is observable.
func TestHandleModConsent_RequestsOnlyTheDelegatedModerationScopes(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformTwitch, "mod-user-1", "delete")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	scopes := strings.Fields(authURLFrom(t, w).Query().Get("scope"))
	assert.Equal(t, []string{"moderator:manage:chat_messages"}, scopes)
	assert.NotContains(t, scopes, oauth.TwitchSendScope,
		"a moderator must not be granted chat-send in v1")
	assert.NotContains(t, scopes, "channel:read:subscriptions",
		"the streamer login scopes must not appear on a volunteer's consent screen")
}

func TestHandleModConsent_ScopesFollowTheRequestedActions(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformTwitch, "mod-user-1", "timeout,ban")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	scopes := strings.Fields(authURLFrom(t, w).Query().Get("scope"))
	assert.Contains(t, scopes, "moderator:manage:banned_users")
	assert.NotContains(t, scopes, "moderator:manage:chat_messages",
		"delete was not requested, so its scope must not be asked for")
}

// The state must be a mod-consent state, not an add-source one — that is what keeps the callback
// away from addSourceToOverlay, which 404s for a non-owner.
func TestHandleModConsent_StashesAModConsentState(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformTwitch, "mod-user-1", "delete")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	state, err := oauth.DecodeOAuthState(authURLFrom(t, w).Query().Get("state"))
	require.NoError(t, err)

	assert.True(t, state.IsModConsent())
	assert.False(t, state.IsAddSource(),
		"an add-source state would route the callback into addSourceToOverlay")
	assert.Equal(t, "mod-user-1", state.UserID)
	assert.Empty(t, state.OverlayID,
		"consent is per platform and account-wide, so it must not be bound to an overlay")
}

// This grants a capability to an existing account and must never create one.
func TestHandleModConsent_RequiresAuthentication(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformTwitch, "", "delete")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// "engagement" is accepted by the shared scope mapper (it maps to channel:read:polls /
// channel:read:predictions) but is NOT delegatable — polls and predictions stay owner-only. A
// crafted URL must not be able to widen a volunteer's consent screen to scopes on their own
// channel that delegation never uses.
func TestHandleModConsent_RejectsNoValidActions(t *testing.T) {
	h := modConsentTestHandler(t)

	for _, actions := range []string{"", "nonsense", "engagement"} {
		t.Run("actions="+actions, func(t *testing.T) {
			w := startModConsent(t, h, oauth.PlatformTwitch, "mod-user-1", actions)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

// A mix of delegatable and non-delegatable actions keeps only the former, rather than refusing
// outright or letting the extra scopes through.
func TestHandleModConsent_DropsNonDelegatableActionsFromAMix(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformTwitch, "mod-user-1", "delete,engagement")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	scopes := strings.Fields(authURLFrom(t, w).Query().Get("scope"))
	assert.Equal(t, []string{"moderator:manage:chat_messages"}, scopes)
	for _, engagement := range []string{"channel:read:polls", "channel:read:predictions"} {
		assert.NotContains(t, scopes, engagement,
			"engagement scopes are not delegatable and must never reach a moderator's consent screen")
	}
}

// --- Kick ------------------------------------------------------------------
//
// Kick's leg differs from Twitch's in two visible ways: PKCE is mandatory, and the consent screen
// legitimately carries `user:read` — the identity read that tells the callback which Kick account
// consented, without which the credential cannot be attributed to anyone.

func TestHandleModConsent_KickRequestsIdentityPlusTheDelegatedScopes(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformKick, "mod-user-1", "ban")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	scopes := strings.Fields(authURLFrom(t, w).Query().Get("scope"))
	assert.ElementsMatch(t, []string{"moderation:ban", "user:read"}, scopes)
	assert.NotContains(t, scopes, oauth.KickSendScope,
		"a moderator must not be granted chat-send in v1")
}

// Kick grants delete separately from ban, so a delete-only delegation must ask for the message
// scope alone — asking for both would over-request on a volunteer's screen.
func TestHandleModConsent_KickDeleteAsksForTheMessageScope(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformKick, "mod-user-1", "delete")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	scopes := strings.Fields(authURLFrom(t, w).Query().Get("scope"))
	assert.ElementsMatch(t, []string{"moderation:chat_message:manage", "user:read"}, scopes)
}

// Without the stashed verifier the callback's token exchange fails, so the consent screen would
// send the moderator on a round trip that could never complete.
func TestHandleModConsent_KickStashesThePKCEVerifier(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformKick, "mod-user-1", "ban")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	u := authURLFrom(t, w)
	assert.Equal(t, "S256", u.Query().Get("code_challenge_method"), "Kick requires PKCE")
	assert.NotEmpty(t, u.Query().Get("code_challenge"))

	state, err := oauth.DecodeOAuthState(u.Query().Get("state"))
	require.NoError(t, err)
	assert.True(t, state.IsModConsent())

	verifier, err := h.redis.Get(context.Background(),
		"oauth_verifier:kick:"+state.CSRFToken).Result()
	require.NoError(t, err, "the callback reads the verifier under this exact key")
	assert.NotEmpty(t, verifier)
}

// A platform whose leg has not landed must say so rather than issue a consent screen that cannot
// be completed. YouTube is that platform until its write path is finished.
func TestHandleModConsent_UnlandedPlatformIsRefused(t *testing.T) {
	h := modConsentTestHandler(t)
	h.providers[oauth.PlatformYouTube] = oauth.NewYouTubeOAuth("yt-id", "yt-secret", "https://allch.at/cb")

	w := startModConsent(t, h, oauth.PlatformYouTube, "mod-user-1", "ban")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not available yet")
}

// A platform with no provider configured at all is a different refusal from an unlanded leg.
func TestHandleModConsent_UnknownPlatformIsRefused(t *testing.T) {
	h := modConsentTestHandler(t)

	w := startModConsent(t, h, oauth.PlatformYouTube, "mod-user-1", "ban")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not supported")
}
