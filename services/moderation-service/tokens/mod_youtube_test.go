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

// The YouTube half of ADR-0048's delegated write: the moderator's own credential, and the
// owner-reach anchor.
//
// YouTube's anchor is unusual in two ways. It yields no id — a write is addressed by the
// broadcast's liveChatId, so there is no broadcaster id to carry — and it must NOT reuse Resolve's
// fallback to the channel-agnostic `users` row, which would match any channel id and so assert
// exactly the thing an anchor cannot see: WHICH channel the user controls.

func TestModYouTubeResolve_ReturnsTheModeratorsOwnCredential(t *testing.T) {
	_, cipher, pool, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()
	src := NewModYouTubeSource(pool, cipher, "test-client-id", "test-client-secret", "")

	cred, err := src.Resolve(context.Background(), ytModUser)

	require.NoError(t, err)
	assert.Equal(t, "ytModAcc", cred.AccessToken, "decrypted, not the stored ciphertext")
	assert.Equal(t, "ytModRef", cred.RefreshToken)
	assert.Equal(t, "google-9001", cred.PlatformUserID, "the Google account id, recorded for attribution")
	assert.Equal(t, []string{ytForceSSL}, cred.GrantedScopes)
}

// Not having consented yet is the normal state of a fresh grant (consent is deferred to first use).
func TestModYouTubeResolve_NoConsentIsNoCredential(t *testing.T) {
	_, cipher, pool, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()
	src := NewModYouTubeSource(pool, cipher, "test-client-id", "test-client-secret", "")

	_, err := src.Resolve(context.Background(), ytStrangerF)

	assert.ErrorIs(t, err, ErrNoCredential)
}

// A streamer's own broadcaster credential is not a moderator credential, and the two tables must
// not be read across — that is the fallback the design forbids.
func TestModYouTubeResolve_DoesNotSeeBroadcasterCredentials(t *testing.T) {
	_, cipher, pool, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()
	src := NewModYouTubeSource(pool, cipher, "test-client-id", "test-client-secret", "")

	_, err := src.Resolve(context.Background(), ytUserY)

	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestModYouTubeRefresh_PersistsAndKeepsTheRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "ytModRef", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		// Google does not reissue the refresh token on refresh.
		_, _ = w.Write([]byte(`{"access_token":"ytModAcc2","expires_in":3599}`))
	}))
	defer srv.Close()

	_, cipher, pool, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()
	src := NewModYouTubeSource(pool, cipher, "test-client-id", "test-client-secret", srv.URL)

	cred, err := src.Resolve(context.Background(), ytModUser)
	require.NoError(t, err)

	require.NoError(t, src.Refresh(context.Background(), ytModUser, cred))
	assert.Equal(t, "ytModAcc2", cred.AccessToken)
	assert.Equal(t, "ytModRef", cred.RefreshToken,
		"Google omits the refresh token, and dropping the stored one would end the credential's life")
	assert.WithinDuration(t, time.Now().Add(time.Hour), cred.ExpiresAt, 2*time.Minute)

	reread, err := src.Resolve(context.Background(), ytModUser)
	require.NoError(t, err)
	assert.Equal(t, "ytModAcc2", reread.AccessToken, "re-encrypted and persisted")
	assert.Equal(t, "ytModRef", reread.RefreshToken)
	assert.Equal(t, []string{ytForceSSL}, reread.GrantedScopes, "a refresh never touches the granted scopes")
}

// --- Owner-reach anchor ------------------------------------------------------

// The per-channel token row IS the evidence: it exists because Google issued a token for that
// channel's own account.
func TestOwnerYouTubeAnchor_AcceptsAPerChannelCredential(t *testing.T) {
	src, _, _, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()

	require.NoError(t, src.OwnerYouTubeAnchor(context.Background(), ytUserY, "UCself"))
	require.NoError(t, src.OwnerYouTubeAnchor(context.Background(), ytLinkedT, "UClinked"),
		"a linked YouTube channel proves control just as well")
}

// The load-bearing case: a YouTube-login streamer's `users` row is channel-agnostic, so Resolve
// happily returns a credential for ANY channel id. The anchor must not inherit that — otherwise it
// would "verify" control of a channel the streamer merely added as a read-only source, which is the
// one thing it exists to rule out.
func TestOwnerYouTubeAnchor_RejectsTheChannelAgnosticUsersRow(t *testing.T) {
	src, _, _, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()

	// Resolve succeeds for a channel this user does not control...
	_, err := src.Resolve(context.Background(), ytUserY, "UCsomeoneElsesChannel")
	require.NoError(t, err, "the users-row fallback matches any channel id — this is the hazard")

	// ...and the anchor must still refuse it.
	assert.ErrorIs(t,
		src.OwnerYouTubeAnchor(context.Background(), ytUserY, "UCsomeoneElsesChannel"),
		ErrOwnerChannelUnverified)
}

// The anchor is about a specific owner: another account's credential for the same channel proves
// nothing about this one.
func TestOwnerYouTubeAnchor_IsScopedToTheOwner(t *testing.T) {
	src, _, _, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()

	assert.ErrorIs(t,
		src.OwnerYouTubeAnchor(context.Background(), ytStrangerF, "UCself"),
		ErrOwnerChannelUnverified)
	assert.ErrorIs(t,
		src.OwnerYouTubeAnchor(context.Background(), ytLinkedT, "UCself"),
		ErrOwnerChannelUnverified)
}

// The anchor proves CONTROL, never capability: a row with no moderation scope still anchors, because
// requiring one would deny delegation to exactly the streamer who delegates because they do not
// moderate themselves.
func TestOwnerYouTubeAnchor_AppliesNoScopePredicate(t *testing.T) {
	src, _, pool, cleanup := setupYouTubeSourceWithPool(t)
	defer cleanup()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry, granted_scopes, encryption_version)
		 VALUES ($1,'UCnoscope','x','y',NOW() + INTERVAL '1 hour','{}',1)`, ytLinkedT)
	require.NoError(t, err)

	assert.NoError(t, src.OwnerYouTubeAnchor(context.Background(), ytLinkedT, "UCnoscope"))
}
