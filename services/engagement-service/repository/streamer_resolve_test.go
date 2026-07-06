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

//go:build integration

// Integration coverage for ADR-0031's username→overlay resolution
// (PublicOverlayForStreamer): the streamer-keyed viewer endpoints depend on it
// matching the api-gateway's /ws/chat/{streamer} resolution exactly — only an
// active, public-for-viewers, non-banned streamer's overlay resolves. Reuses the
// newTestDB helper in idor_test.go. Run: go test -tags=integration ./repository/...
package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedStreamer creates a throwaway user with a known username plus one overlay whose
// active/public/banned flags are set explicitly, and returns (username, overlayID).
func seedStreamer(t *testing.T, pool *pgxpool.Pool, active, public, banned bool) (string, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	username := "res-" + suffix
	var userID, overlayID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at, is_banned)
		 VALUES ($1, $2, $3, 'x', 'x', NOW() + INTERVAL '1 day', $4) RETURNING id`,
		"res-"+suffix, username, "Res "+suffix, banned).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO overlays (user_id, name, is_active, is_public_for_viewers)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, "res-overlay-"+suffix, active, public).Scan(&overlayID))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })
	return username, overlayID
}

// TestPublicOverlayForStreamer covers the resolution ADR-0031's endpoints rely on: a
// public, active, non-banned streamer resolves to their overlay; every other case
// (private, inactive, banned, unknown) resolves to ok=false so the handler 404s and
// never leaks whether a username exists.
func TestPublicOverlayForStreamer(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	t.Run("public active non-banned resolves", func(t *testing.T) {
		username, overlayID := seedStreamer(t, pool, true, true, false)
		got, ok, err := repo.PublicOverlayForStreamer(ctx, username)
		require.NoError(t, err)
		assert.True(t, ok, "a public, active, non-banned streamer must resolve")
		assert.Equal(t, overlayID, got)
	})

	t.Run("private overlay does not resolve", func(t *testing.T) {
		username, _ := seedStreamer(t, pool, true, false, false)
		_, ok, err := repo.PublicOverlayForStreamer(ctx, username)
		require.NoError(t, err)
		assert.False(t, ok, "an overlay not public-for-viewers must not resolve")
	})

	t.Run("inactive overlay does not resolve", func(t *testing.T) {
		username, _ := seedStreamer(t, pool, false, true, false)
		_, ok, err := repo.PublicOverlayForStreamer(ctx, username)
		require.NoError(t, err)
		assert.False(t, ok, "an inactive overlay must not resolve")
	})

	t.Run("banned streamer does not resolve", func(t *testing.T) {
		username, _ := seedStreamer(t, pool, true, true, true)
		_, ok, err := repo.PublicOverlayForStreamer(ctx, username)
		require.NoError(t, err)
		assert.False(t, ok, "a banned streamer must not resolve")
	})

	t.Run("unknown username does not resolve", func(t *testing.T) {
		_, ok, err := repo.PublicOverlayForStreamer(ctx, "no-such-streamer-"+uuid.NewString())
		require.NoError(t, err)
		assert.False(t, ok, "an unknown username must resolve to ok=false, not an error")
	})
}
