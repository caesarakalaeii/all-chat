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

package tokens

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Kick half of ADR-0048's delegated write: the moderator's own credential, and the
// owner-reach anchor that supplies broadcaster_user_id.
//
// Kick makes the split starker than Twitch does. Its API carries no moderator field at all —
// the acting identity is whoever the token belongs to — so `broadcaster_user_id` is the ONLY
// id in the request, and it must come from the owner. Sourcing it from the moderator's own
// credential (which is what a naive resolve does) would silently moderate the moderator's own
// channel instead of the streamer's.

func TestModKickResolve_ReturnsTheModeratorsOwnCredential(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	cred, err := src.Resolve(context.Background(), kickModUser)

	require.NoError(t, err)
	assert.Equal(t, "kmodAcc", cred.AccessToken, "decrypted, not the stored ciphertext")
	assert.Equal(t, "kmodRef", cred.RefreshToken)
	assert.Equal(t, "9001", cred.PlatformUserID, "the moderator's own numeric Kick id")
	assert.Equal(t, []string{"user:read", "moderation:ban"}, cred.GrantedScopes)
}

// One row per (user, platform), so a volunteer who moderates on both platforms must get the
// Kick one here. Reading the wrong row would hand Kick a Twitch access token.
func TestModKickResolve_IsScopedToKick(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	cred, err := src.Resolve(context.Background(), kickModUser)

	require.NoError(t, err)
	assert.Equal(t, "kmodAcc", cred.AccessToken)
	assert.NotEqual(t, "twitchModAcc", cred.AccessToken, "the Twitch moderator row must not be read")
}

// Not having consented yet is the normal state of a fresh grant (consent is deferred to first
// use), so it is a sentinel the dispatcher acts on rather than an error.
func TestModKickResolve_NoConsentIsNoCredential(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	_, err := src.Resolve(context.Background(), kickStrangerE)

	assert.ErrorIs(t, err, ErrNoCredential)
}

// A streamer's own broadcaster credential is not a moderator credential. Kick keeps them in
// different tables for the same reason Twitch does: kick-listener selects ingest credentials by
// channel with no user scoping, so a moderator-scoped row in kick_oauth_tokens could become a
// candidate ingest credential and break chat on a real channel.
func TestModKickResolve_DoesNotSeeBroadcasterCredentials(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	_, err := src.Resolve(context.Background(), kickUserK)

	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestModKickRefresh_PersistsToTheModeratorsOwnRow(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "kmodRef", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"kmodAcc2","refresh_token":"kmodRef2","expires_in":3600}`))
	}))
	defer srv.Close()
	src.refresh.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), kickModUser)
	require.NoError(t, err)

	require.NoError(t, src.Refresh(context.Background(), kickModUser, cred))
	assert.Equal(t, "kmodAcc2", cred.AccessToken)
	assert.Equal(t, "kmodRef2", cred.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), cred.ExpiresAt, time.Minute)

	reread, err := src.Resolve(context.Background(), kickModUser)
	require.NoError(t, err)
	assert.Equal(t, "kmodAcc2", reread.AccessToken, "re-encrypted and persisted")
	assert.Equal(t, "kmodRef2", reread.RefreshToken)
}

// Kick may omit the refresh token on a refresh response; dropping the stored one would end the
// credential's life at the next expiry.
func TestModKickRefresh_KeepsTheOldRefreshTokenWhenNotRotated(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"kmodAcc3","expires_in":3600}`))
	}))
	defer srv.Close()
	src.refresh.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), kickModUser)
	require.NoError(t, err)
	require.NoError(t, src.Refresh(context.Background(), kickModUser, cred))

	assert.Equal(t, "kmodRef", cred.RefreshToken, "the stored refresh token survives a non-rotating response")
	reread, err := src.Resolve(context.Background(), kickModUser)
	require.NoError(t, err)
	assert.Equal(t, "kmodRef", reread.RefreshToken)
}

// The scopes are owned by the consent flow; a refresh grant never widens them, so it must not
// touch them either.
func TestModKickRefresh_LeavesGrantedScopesAlone(t *testing.T) {
	_, cipher, pool, cleanup := setupKickSourceWithPool(t)
	defer cleanup()
	src := NewModKickSource(pool, cipher, "test-client-id", "test-client-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"b","expires_in":3600}`))
	}))
	defer srv.Close()
	src.refresh.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), kickModUser)
	require.NoError(t, err)
	require.NoError(t, src.Refresh(context.Background(), kickModUser, cred))

	reread, err := src.Resolve(context.Background(), kickModUser)
	require.NoError(t, err)
	assert.Equal(t, []string{"user:read", "moderation:ban"}, reread.GrantedScopes)
}

// --- Owner-reach anchor ------------------------------------------------------

// A Kick source's channel_id is the channel slug, and the anchor turns it into the numeric
// broadcaster_user_id the moderation API needs — the one id a moderator's credential cannot
// supply.
func TestOwnerKickAnchor_ResolvesTheBroadcasterID(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerKickAnchor(context.Background(), kickUserK, "kickstreamer")

	require.NoError(t, err)
	assert.Equal(t, "555", id)
}

func TestOwnerKickAnchor_IsCaseInsensitive(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerKickAnchor(context.Background(), kickUserK, "KickStreamer")

	require.NoError(t, err)
	assert.Equal(t, "555", id)
}

// A linked (non-primary-login) Kick account proves control just as well — the streamer connected
// it, which is the whole claim being made.
func TestOwnerKickAnchor_AcceptsALinkedCredential(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerKickAnchor(context.Background(), kickLinkedT, "linkedkick")

	require.NoError(t, err)
	assert.Equal(t, "777", id)
}

// The anchor is about a specific owner. Another account's credential for the same channel proves
// nothing about this one — otherwise any user could lend their channel to someone else's overlay.
func TestOwnerKickAnchor_IsScopedToTheOwner(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	_, err := src.OwnerKickAnchor(context.Background(), kickStrangerE, "kickstreamer")

	assert.ErrorIs(t, err, ErrOwnerChannelUnverified)
}

func TestOwnerKickAnchor_UnknownChannelIsUnverified(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	_, err := src.OwnerKickAnchor(context.Background(), kickUserK, "some-channel-they-do-not-own")

	assert.ErrorIs(t, err, ErrOwnerChannelUnverified)
}

// A legacy listener row carries no kick_user_id, so it cannot yield a broadcaster id. Anchoring on
// it would produce an empty id and a malformed API call rather than an honest refusal.
func TestOwnerKickAnchor_LegacyListenerRowDoesNotAnchor(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	_, err := src.OwnerKickAnchor(context.Background(), kickLegacyL, "legacychannel")

	assert.ErrorIs(t, err, ErrOwnerChannelUnverified)
}

// The anchor proves CONTROL, never capability. Requiring a moderation scope would deny delegation
// to exactly the streamer who delegates because they do not moderate themselves.
func TestOwnerKickAnchor_AppliesNoScopePredicate(t *testing.T) {
	src, _, _, cleanup := setupKickSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerKickAnchor(context.Background(), kickNoScopeN, "noscopestreamer")

	require.NoError(t, err)
	assert.Equal(t, "888", id, "a login-only grant still anchors the channel")
}
