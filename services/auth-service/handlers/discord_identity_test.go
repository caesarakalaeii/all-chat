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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// identityHandler builds a DiscordHandler with a usable in-memory state store.
func identityHandler(o *mockDiscordOAuth, r *mockDiscordRepo) *DiscordHandler {
	return newTestDiscordHandlerNoRedis(o, r, "https://allch.at")
}

// authed runs a request through a gin context that already carries a user id.
func authed(h *DiscordHandler, method, target, userID string, fn func(*DiscordHandler, *gin.Context)) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	if userID != "" {
		c.Set("user_id", userID)
	}
	fn(h, c)
	return w
}

func TestHandleIdentityConnect_ReturnsTheIdentifyURLAndStoresAnIdentityState(t *testing.T) {
	o, r := &mockDiscordOAuth{}, &mockDiscordRepo{}
	h := identityHandler(o, r)
	store := h.stateStore.(*memStateStore)

	w := authed(h, http.MethodGet, "/discord/identity/connect?return=moderate", "user-1",
		(*DiscordHandler).HandleIdentityConnect)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body["auth_url"], "scope=identify")

	require.Len(t, store.states, 1, "exactly one state is issued")
	for _, stored := range store.states {
		flow := parseDiscordFlowState(stored)
		assert.Equal(t, discordFlowIdentity, flow.Kind)
		assert.Equal(t, "user-1", flow.UserID)
		assert.Equal(t, "moderate", flow.Return)
	}
}

// TestHandleIdentityConnect_RejectsAnUnknownReturn: the return target is an allowlisted KEY, so a
// caller-supplied path can never become an open redirect.
func TestHandleIdentityConnect_RejectsAnUnknownReturn(t *testing.T) {
	h := identityHandler(&mockDiscordOAuth{}, &mockDiscordRepo{})
	store := h.stateStore.(*memStateStore)

	w := authed(h, http.MethodGet, "/discord/identity/connect?return=https://evil.example/steal", "user-1",
		(*DiscordHandler).HandleIdentityConnect)

	require.Equal(t, http.StatusOK, w.Code)
	for _, stored := range store.states {
		assert.Equal(t, defaultDiscordReturn, parseDiscordFlowState(stored).Return,
			"an unrecognised return key falls back to the default rather than being honoured")
	}
}

func TestHandleIdentityConnect_RequiresAuth(t *testing.T) {
	h := identityHandler(&mockDiscordOAuth{}, &mockDiscordRepo{})
	w := authed(h, http.MethodGet, "/discord/identity/connect", "", (*DiscordHandler).HandleIdentityConnect)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// callback drives the shared callback with a pre-seeded state.
func callback(t *testing.T, h *DiscordHandler, stored, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, h.stateStore.Set(context.Background(), "st-1", stored, time.Minute))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/discord/callback?state=st-1"+query, nil)
	h.HandleCallback(c)
	return w
}

func identityState(t *testing.T, userID, returnKey string) string {
	t.Helper()
	stored, err := encodeDiscordFlowState(discordFlowState{Kind: discordFlowIdentity, UserID: userID, Return: returnKey})
	require.NoError(t, err)
	return stored
}

func TestCallback_IdentityFlowStoresTheLinkAndRedirects(t *testing.T) {
	o := &mockDiscordOAuth{
		exchangeToken: &oauth2.Token{AccessToken: "user-token"},
		identity:      &oauth.DiscordIdentity{ID: "42424242", Username: "volunteer"},
	}
	r := &mockDiscordRepo{}
	h := identityHandler(o, r)

	w := callback(t, h, identityState(t, "user-1", "moderate"), "&code=abc")

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://allch.at/moderate?discord_account=linked", w.Header().Get("Location"))
	assert.True(t, r.upsertIdentityCalled)
	assert.Equal(t, "user-1", r.upsertIdentityUser)
	assert.Equal(t, "42424242", r.upsertIdentityDiscID)
	assert.False(t, r.upsertCalled, "an account link must never create a guild record")
}

// TestCallback_IdentityFlowNeedsNoGuildID is the fork's whole point: an identify callback carries
// no guild_id, and the old handler rejected any callback without one.
func TestCallback_IdentityFlowNeedsNoGuildID(t *testing.T) {
	o := &mockDiscordOAuth{exchangeToken: &oauth2.Token{AccessToken: "t"}}
	r := &mockDiscordRepo{}
	h := identityHandler(o, r)

	w := callback(t, h, identityState(t, "user-1", "settings"), "&code=abc")

	assert.Equal(t, http.StatusFound, w.Code)
	assert.True(t, r.upsertIdentityCalled)
}

// TestCallback_BotInviteStillRequiresAGuildID: the fork must not loosen the invite branch.
func TestCallback_BotInviteStillRequiresAGuildID(t *testing.T) {
	h := identityHandler(&mockDiscordOAuth{exchangeToken: &oauth2.Token{}}, &mockDiscordRepo{})

	w := callback(t, h, "user-1", "&code=abc")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCallback_LegacyBareUserIDStateIsStillABotInvite: state entries live ten minutes, so an
// invite already in flight when this change deploys must still complete.
func TestCallback_LegacyBareUserIDStateIsStillABotInvite(t *testing.T) {
	o := &mockDiscordOAuth{exchangeToken: &oauth2.Token{}}
	r := &mockDiscordRepo{}
	h := identityHandler(o, r)

	w := callback(t, h, "user-1", "&code=abc&guild_id=g-1")

	assert.Equal(t, http.StatusFound, w.Code)
	assert.True(t, r.upsertCalled, "the guild is stored, as before")
	assert.False(t, r.upsertIdentityCalled, "and no identity is invented for it")
}

// TestCallback_FlowKindComesFromServerStateNotTheQuery: a client that could pick the branch could
// redirect a bot invite's code into the link path. Passing guild_id on an identity state must not
// turn it into an invite, and the reverse must not turn an invite into a link.
func TestCallback_FlowKindComesFromServerStateNotTheQuery(t *testing.T) {
	t.Run("guild_id cannot promote an identity state to a bot invite", func(t *testing.T) {
		o := &mockDiscordOAuth{exchangeToken: &oauth2.Token{}}
		r := &mockDiscordRepo{}
		h := identityHandler(o, r)

		callback(t, h, identityState(t, "user-1", "settings"), "&code=abc&guild_id=someone-elses-guild")

		assert.False(t, r.upsertCalled, "no guild record may be created from an identity state")
		assert.True(t, r.upsertIdentityCalled)
	})

	t.Run("an invite state is never treated as a link", func(t *testing.T) {
		o := &mockDiscordOAuth{exchangeToken: &oauth2.Token{}}
		r := &mockDiscordRepo{}
		h := identityHandler(o, r)

		callback(t, h, "user-1", "&code=abc&guild_id=g-1")

		assert.False(t, r.upsertIdentityCalled)
	})
}

func TestCallback_IdentityFlowFailureRedirectsWithACode(t *testing.T) {
	tests := []struct {
		name    string
		oauth   *mockDiscordOAuth
		repo    *mockDiscordRepo
		wantErr string
	}{
		{
			name:    "code exchange failed",
			oauth:   &mockDiscordOAuth{exchangeErr: errors.New("boom")},
			repo:    &mockDiscordRepo{},
			wantErr: "error=exchange_failed",
		},
		{
			name:    "identity unreadable",
			oauth:   &mockDiscordOAuth{exchangeToken: &oauth2.Token{}, identityErr: errors.New("boom")},
			repo:    &mockDiscordRepo{},
			wantErr: "error=identity_unavailable",
		},
		{
			name:    "already linked to another All-Chat user",
			oauth:   &mockDiscordOAuth{exchangeToken: &oauth2.Token{}},
			repo:    &mockDiscordRepo{upsertIdentityErr: repository.ErrDiscordIdentityClaimed},
			wantErr: "error=already_linked",
		},
		{
			name:    "storage failed",
			oauth:   &mockDiscordOAuth{exchangeToken: &oauth2.Token{}},
			repo:    &mockDiscordRepo{upsertIdentityErr: errors.New("db down")},
			wantErr: "error=save_failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := identityHandler(tc.oauth, tc.repo)

			w := callback(t, h, identityState(t, "user-1", "moderate"), "&code=abc")

			assert.Equal(t, http.StatusFound, w.Code, "the user is mid-OAuth in a browser, so failures redirect")
			loc := w.Header().Get("Location")
			assert.True(t, strings.HasPrefix(loc, "https://allch.at/moderate?"), "back where they started: %s", loc)
			assert.Contains(t, loc, tc.wantErr)
		})
	}
}

// TestCallback_IdentityStateWithoutAUserIsRejected: an unparseable or user-less state must not act
// for a user we cannot name.
func TestCallback_IdentityStateWithoutAUserIsRejected(t *testing.T) {
	r := &mockDiscordRepo{}
	h := identityHandler(&mockDiscordOAuth{exchangeToken: &oauth2.Token{}}, r)

	w := callback(t, h, `{"kind":"identity"}`, "&code=abc")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, r.upsertIdentityCalled)
}

func TestParseDiscordFlowState_UnparseableJSONNamesNobody(t *testing.T) {
	flow := parseDiscordFlowState(`{"kind":`)
	assert.Empty(t, flow.UserID, "a broken state must not fall back to treating the blob as a user id")
	assert.Empty(t, flow.Kind)
}

func TestHandleGetIdentity(t *testing.T) {
	t.Run("unlinked", func(t *testing.T) {
		h := identityHandler(&mockDiscordOAuth{}, &mockDiscordRepo{})
		w := authed(h, http.MethodGet, "/discord/identity", "user-1", (*DiscordHandler).HandleGetIdentity)

		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, false, body["linked"])
		assert.NotContains(t, body, "discord_user_id")
	})

	t.Run("linked", func(t *testing.T) {
		r := &mockDiscordRepo{getIdentity: &models.DiscordIdentity{
			UserID: "user-1", DiscordUserID: "42424242", DiscordUsername: "volunteer", LinkedAt: time.Now().UTC(),
		}}
		h := identityHandler(&mockDiscordOAuth{}, r)
		w := authed(h, http.MethodGet, "/discord/identity", "user-1", (*DiscordHandler).HandleGetIdentity)

		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, true, body["linked"])
		assert.Equal(t, "42424242", body["discord_user_id"])
		assert.Equal(t, "volunteer", body["discord_username"])
	})

	t.Run("lookup failure is not reported as unlinked", func(t *testing.T) {
		r := &mockDiscordRepo{getIdentityErr: errors.New("db down")}
		h := identityHandler(&mockDiscordOAuth{}, r)
		w := authed(h, http.MethodGet, "/discord/identity", "user-1", (*DiscordHandler).HandleGetIdentity)

		assert.Equal(t, http.StatusInternalServerError, w.Code,
			"reporting unlinked would show a Connect button to someone already linked")
	})
}

func TestHandleDeleteIdentity(t *testing.T) {
	r := &mockDiscordRepo{}
	h := identityHandler(&mockDiscordOAuth{}, r)

	w := authed(h, http.MethodDelete, "/discord/identity", "user-1", (*DiscordHandler).HandleDeleteIdentity)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, r.deleteIdentityCalled)
}

func TestHandleDeleteIdentity_RequiresAuth(t *testing.T) {
	r := &mockDiscordRepo{}
	h := identityHandler(&mockDiscordOAuth{}, r)

	w := authed(h, http.MethodDelete, "/discord/identity", "", (*DiscordHandler).HandleDeleteIdentity)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, r.deleteIdentityCalled)
}
