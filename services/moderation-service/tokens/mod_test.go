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

// The delegated moderator's own credential, and the owner-reach anchor (ADR-0048).
//
// These are the two halves of a delegated write: the moderator supplies the token and the
// moderator_id, the owner supplies the broadcaster_id — and neither may substitute for the other.

func TestModResolve_ReturnsTheModeratorsOwnCredential(t *testing.T) {
	_, cipher, pool, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()
	src := NewModTwitchSource(pool, cipher, "test-client-id", "test-client-secret")

	cred, err := src.Resolve(context.Background(), modUser)

	require.NoError(t, err)
	assert.Equal(t, "modAcc", cred.AccessToken, "decrypted, not the stored ciphertext")
	assert.Equal(t, "modRef", cred.RefreshToken)
	assert.Equal(t, "7007", cred.PlatformUserID, "the id sent as moderator_id")
	assert.Equal(t, []string{"moderator:manage:chat_messages"}, cred.GrantedScopes)
}

// Keyed on the moderator alone, with no channel: Twitch's moderation scopes are role-based, so
// one consent serves every streamer who delegated Twitch to them.
func TestModResolve_IsNotScopedToAChannel(t *testing.T) {
	_, cipher, pool, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()
	src := NewModTwitchSource(pool, cipher, "test-client-id", "test-client-secret")

	first, err := src.Resolve(context.Background(), modUser)
	require.NoError(t, err)
	second, err := src.Resolve(context.Background(), modUser)
	require.NoError(t, err)
	assert.Equal(t, first.AccessToken, second.AccessToken)
}

// Not having consented yet is the normal state of a fresh grant (consent is deferred to first
// use), so it is a sentinel the dispatcher can act on rather than an error.
func TestModResolve_NoConsentIsNoCredential(t *testing.T) {
	_, cipher, pool, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()
	src := NewModTwitchSource(pool, cipher, "test-client-id", "test-client-secret")

	_, err := src.Resolve(context.Background(), strangerD)

	assert.ErrorIs(t, err, ErrNoCredential)
}

// A streamer's own broadcaster credential is NOT a moderator credential. They live in different
// tables on purpose, and reading across would reintroduce the fallback the design forbids.
func TestModResolve_DoesNotSeeBroadcasterCredentials(t *testing.T) {
	_, cipher, pool, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()
	src := NewModTwitchSource(pool, cipher, "test-client-id", "test-client-secret")

	// userA holds a perfectly good broadcaster credential and no moderator credential.
	_, err := src.Resolve(context.Background(), userA)

	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestModRefresh_PersistsToTheModeratorsOwnRow(t *testing.T) {
	_, cipher, pool, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()
	src := NewModTwitchSource(pool, cipher, "test-client-id", "test-client-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "modRef", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"modAcc2","refresh_token":"modRef2","expires_in":3600}`))
	}))
	defer srv.Close()
	src.refresh.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), modUser)
	require.NoError(t, err)

	require.NoError(t, src.Refresh(context.Background(), modUser, cred))
	assert.Equal(t, "modAcc2", cred.AccessToken)
	assert.Equal(t, "modRef2", cred.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), cred.ExpiresAt, time.Minute)

	reread, err := src.Resolve(context.Background(), modUser)
	require.NoError(t, err)
	assert.Equal(t, "modAcc2", reread.AccessToken, "re-encrypted and persisted")
	assert.Equal(t, "modRef2", reread.RefreshToken)
}

// The scopes are owned by the consent flow; a refresh grant never widens them, so it must not
// touch them either.
func TestModRefresh_LeavesGrantedScopesAlone(t *testing.T) {
	_, cipher, pool, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()
	src := NewModTwitchSource(pool, cipher, "test-client-id", "test-client-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"b","expires_in":3600}`))
	}))
	defer srv.Close()
	src.refresh.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), modUser)
	require.NoError(t, err)
	require.NoError(t, src.Refresh(context.Background(), modUser, cred))

	reread, err := src.Resolve(context.Background(), modUser)
	require.NoError(t, err)
	assert.Equal(t, []string{"moderator:manage:chat_messages"}, reread.GrantedScopes)
}

// --- Owner-reach anchor ------------------------------------------------------

// The anchor yields the numeric broadcaster id for a channel the owner controls. A Twitch
// source's channel_id IS the login, which is what makes this answerable.
func TestOwnerAnchor_ResolvesTheBroadcasterID(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerTwitchAnchor(context.Background(), userA, "streamerA")

	require.NoError(t, err)
	assert.Equal(t, "1001", id)
}

func TestOwnerAnchor_IsCaseInsensitive(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerTwitchAnchor(context.Background(), userA, "StReAmErA")

	require.NoError(t, err)
	assert.Equal(t, "1001", id)
}

// A linked (non-primary-login) Twitch account proves control just as well — the streamer connected
// it, which is the whole claim being made.
func TestOwnerAnchor_AcceptsALinkedCredential(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerTwitchAnchor(context.Background(), userB, "streamerB")

	require.NoError(t, err)
	assert.Equal(t, "2002", id)
}

// ADR-0016's preference, so the anchor and the owner's own credential resolution never disagree
// about which id a channel has.
func TestOwnerAnchor_PrefersTheUsersRow(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	id, err := src.OwnerTwitchAnchor(context.Background(), userC, "dualstreamer")

	require.NoError(t, err)
	assert.Equal(t, "3003", id, "the users row wins over a stale linked row")
}

// The anchor is about a specific owner. Another account's credential for the same channel proves
// nothing about this one — otherwise any user could lend their channel to someone else's overlay.
func TestOwnerAnchor_IsScopedToTheOwner(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	_, err := src.OwnerTwitchAnchor(context.Background(), strangerD, "streamerA")

	assert.ErrorIs(t, err, ErrOwnerChannelUnverified)
}

func TestOwnerAnchor_UnknownChannelIsUnverified(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	_, err := src.OwnerTwitchAnchor(context.Background(), userA, "some-channel-they-do-not-own")

	assert.ErrorIs(t, err, ErrOwnerChannelUnverified)
}

// The anchor proves CONTROL, never capability. Requiring a moderation scope would deny delegation
// to exactly the streamer who delegates because they do not moderate themselves — and userB's
// linked credential deliberately carries only banned-users scope, not the delete scope.
func TestOwnerAnchor_AppliesNoScopePredicate(t *testing.T) {
	src, _, _, cleanup := setupTokenSourceWithPool(t)
	defer cleanup()

	// userB holds no moderator:manage:chat_messages anywhere, yet still anchors the channel.
	id, err := src.OwnerTwitchAnchor(context.Background(), userB, "streamerB")

	require.NoError(t, err)
	assert.Equal(t, "2002", id)
}
