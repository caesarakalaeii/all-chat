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

// ActivateSourcesForOverlay against a Postgres with every migration applied. CI runs it
// in the test-engagement-integration job; locally point TEST_DATABASE_URL at a migrated
// database and run: go test -tags=integration ./subscription/
package subscription

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A websocket connect re-activates the overlay's sources, but never a shared_overlay
// source whose share was revoked or expired. The source stores the sender overlay id,
// not the share id, so the old share-id match never excluded anything.
func TestActivateSourcesForOverlay_SkipsEndedShares(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://allchat:allchat_dev_password@localhost:5432/allchat?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "integration tests need a migrated Postgres (TEST_DATABASE_URL)")
	var userIDs, overlayIDs []string
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM share_requests WHERE sender_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM overlays WHERE id = ANY($1::uuid[])`, overlayIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
		pool.Close()
	})

	tag := fmt.Sprintf("gw_share_%d", time.Now().UnixNano())
	user := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
			VALUES ($1, $1, $1, 'a', 'r', NOW() + INTERVAL '1 hour') RETURNING id`, tag+name).Scan(&id))
		userIDs = append(userIDs, id)
		return id
	}
	overlay := func(userID string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO overlays (user_id, name, is_public_for_viewers) VALUES ($1, 'share test', false) RETURNING id`, userID).Scan(&id))
		overlayIDs = append(overlayIDs, id)
		return id
	}
	inactiveSource := func(overlayID, platform, channelID string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name, is_active)
			VALUES ($1, $2, $3, 'share test', false) RETURNING id`, overlayID, platform, channelID).Scan(&id))
		return id
	}
	share := func(sender, senderOverlay, recipient, status string) {
		_, err := pool.Exec(ctx, `
			INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, responded_at)
			VALUES ($1, $2, $3, $4, NOW())`, sender, senderOverlay, recipient, status)
		require.NoError(t, err)
	}
	active := func(sourceID string) bool {
		var a bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT is_active FROM overlay_chat_sources WHERE id = $1`, sourceID).Scan(&a))
		return a
	}

	sender, other, recipient := user("s"), user("o"), user("r")
	revokedOverlay, liveOverlay := overlay(sender), overlay(sender)
	// The revoked overlay is still shared with someone else: that share is not the
	// recipient's and must not reactivate the recipient's source.
	share(sender, revokedOverlay, recipient, "revoked")
	share(sender, revokedOverlay, other, "accepted")
	share(sender, liveOverlay, recipient, "accepted")

	recipientOverlay := overlay(recipient)
	twitchSrc := inactiveSource(recipientOverlay, "twitch", tag)
	revokedSrc := inactiveSource(recipientOverlay, "shared_overlay", revokedOverlay)
	liveSrc := inactiveSource(recipientOverlay, "shared_overlay", liveOverlay)

	n, err := NewRepository(pool).ActivateSourcesForOverlay(ctx, recipientOverlay)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.True(t, active(twitchSrc))
	assert.True(t, active(liveSrc), "a source on an accepted share reactivates")
	assert.False(t, active(revokedSrc), "a source on a revoked share stays off")
}
