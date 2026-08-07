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

package oauth

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Delegated-moderator consent (ADR-0048) is its OWN OAuth action, not a variant of add-source.
// The existing moderation re-consent flow (NewModerationState) is deliberately an add-source
// state, and the shared callback calls addSourceToOverlay for every add-source state — which
// 404s for a non-owner. A moderator going through that path would have their credential
// persisted and THEN be redirected to an error.

func TestNewModConsentState_IsNotAnAddSourceState(t *testing.T) {
	s := NewModConsentState("csrf-1", "user-9")

	assert.True(t, s.IsModConsent())
	assert.False(t, s.IsAddSource(),
		"a mod-consent state must never satisfy IsAddSource — that is what routes it into addSourceToOverlay")
	assert.False(t, s.IsLogin())
	assert.False(t, s.IsModeration(),
		"the owner's moderation re-consent purpose must not be confused with a moderator's consent")
	assert.Equal(t, "user-9", s.UserID)
	assert.Empty(t, s.OverlayID,
		"mod consent is per platform and account-wide, so it is not bound to an overlay")
}

func TestModConsentState_Validate(t *testing.T) {
	t.Run("accepted as a valid action", func(t *testing.T) {
		require.NoError(t, NewModConsentState("csrf-1", "user-9").Validate())
	})

	t.Run("requires the moderator's user id", func(t *testing.T) {
		s := NewModConsentState("csrf-1", "")
		err := s.Validate()
		require.Error(t, err, "without a user id there is nobody to attribute the credential to")
		assert.Contains(t, err.Error(), "user_id")
	})

	t.Run("still requires a csrf token", func(t *testing.T) {
		s := NewModConsentState("", "user-9")
		assert.Error(t, s.Validate())
	})
}

func TestModConsentState_RoundTrip(t *testing.T) {
	encoded, err := NewModConsentState("csrf-1", "user-9").Encode()
	require.NoError(t, err)

	decoded, err := DecodeOAuthState(encoded)
	require.NoError(t, err)

	assert.True(t, decoded.IsModConsent())
	assert.False(t, decoded.IsAddSource())
	assert.Equal(t, "user-9", decoded.UserID)
}

// The consent screen a volunteer sees must ask for the moderation scopes and nothing else.
// GetAuthURLWithScopes unconditionally prepends the base login set (channel:read:redemptions,
// channel:read:subscriptions, bits:read, moderator:read:followers), which would ask a moderator
// for channel-point, subscription and bits read on their OWN channel — an ADR-0012 regression.
func TestTwitchOAuth_GetModConsentAuthURL_OmitsBaseLoginScopes(t *testing.T) {
	tw := NewTwitchOAuth("client-id", "client-secret", "https://allch.at/callback")

	raw := tw.GetModConsentAuthURL("state-1", []string{"moderator:manage:chat_messages"})

	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	scopes := strings.Fields(parsed.Query().Get("scope"))

	assert.Equal(t, []string{"moderator:manage:chat_messages"}, scopes)
	for _, base := range []string{"channel:read:redemptions", "channel:read:subscriptions", "bits:read", "moderator:read:followers"} {
		assert.NotContains(t, scopes, base,
			"a moderator's consent screen must not request the streamer login scopes")
	}
}

// force_verify is required for the same reason the owner flow needs it: without it Twitch may
// silently reissue a prior, narrower grant, so the moderation scopes would never be requested.
func TestTwitchOAuth_GetModConsentAuthURL_ForcesVerify(t *testing.T) {
	tw := NewTwitchOAuth("client-id", "client-secret", "https://allch.at/callback")

	parsed, err := url.Parse(tw.GetModConsentAuthURL("state-1", []string{"moderator:manage:banned_users"}))
	require.NoError(t, err)

	assert.Equal(t, "true", parsed.Query().Get("force_verify"))
	assert.Equal(t, "state-1", parsed.Query().Get("state"))
}

// Deduped and stable, so a repeat consent produces the same screen.
func TestTwitchOAuth_GetModConsentAuthURL_DedupesScopes(t *testing.T) {
	tw := NewTwitchOAuth("client-id", "client-secret", "https://allch.at/callback")

	parsed, err := url.Parse(tw.GetModConsentAuthURL("state-1", []string{
		"moderator:manage:chat_messages",
		"moderator:manage:banned_users",
		"moderator:manage:chat_messages",
	}))
	require.NoError(t, err)

	scopes := strings.Fields(parsed.Query().Get("scope"))
	assert.Equal(t, []string{"moderator:manage:chat_messages", "moderator:manage:banned_users"}, scopes)
}

// An empty scope set would produce a consent screen granting nothing, which then fails at the
// first moderation call with a confusing "missing scope". Refuse to build it.
func TestTwitchOAuth_GetModConsentAuthURL_RejectsEmptyScopes(t *testing.T) {
	tw := NewTwitchOAuth("client-id", "client-secret", "https://allch.at/callback")

	assert.Empty(t, tw.GetModConsentAuthURL("state-1", nil),
		"no scopes means no meaningful consent to ask for")
}
