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

// The share fan-out of FindAlertOverlays against a Postgres with every migration
// applied. CI runs it in the test-engagement-integration job; locally point
// TEST_DATABASE_URL at a migrated database and run: go test -tags=integration ./repository/
package repository

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

// Same contract as the message-processor's chat fan-out: a revoked recipient's alerts
// overlay stops receiving the sender's events while the overlay stays shared with others.
func TestFindAlertOverlays_RevokedRecipientStopsReceiving(t *testing.T) {
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

	tag := fmt.Sprintf("ap_share_%d", time.Now().UnixNano())
	user := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
			VALUES ($1, $1, $1, 'a', 'r', NOW() + INTERVAL '1 hour') RETURNING id`, tag+name).Scan(&id))
		userIDs = append(userIDs, id)
		return id
	}
	overlay := func(userID, kind, platform, channelID string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO overlays (user_id, name, overlay_type) VALUES ($1, 'share test', $2) RETURNING id`,
			userID, kind).Scan(&id))
		overlayIDs = append(overlayIDs, id)
		_, err := pool.Exec(ctx, `
			INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name, is_active)
			VALUES ($1, $2, $3, 'share test', true)`, id, platform, channelID)
		require.NoError(t, err)
		return id
	}
	share := func(sender, senderOverlay, recipient string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, responded_at)
			VALUES ($1, $2, $3, 'accepted', NOW()) RETURNING id`, sender, senderOverlay, recipient).Scan(&id))
		return id
	}

	sender, r1, r2 := user("s"), user("r1"), user("r2")
	senderOverlay := overlay(sender, "chat", "twitch", tag)
	r1Overlay := overlay(r1, "alerts", "shared_overlay", senderOverlay)
	r2Overlay := overlay(r2, "alerts", "shared_overlay", senderOverlay)
	share1 := share(sender, senderOverlay, r1)
	share(sender, senderOverlay, r2)

	repo := NewRepository(pool)
	routed := func() []string {
		targets, err := repo.FindAlertOverlays(ctx, "twitch", tag)
		require.NoError(t, err)
		ids := make([]string, 0, len(targets))
		for _, target := range targets {
			ids = append(ids, target.OverlayID)
		}
		return ids
	}

	assert.ElementsMatch(t, []string{r1Overlay, r2Overlay}, routed())

	_, err = pool.Exec(ctx, `UPDATE share_requests SET status = 'revoked' WHERE id = $1`, share1)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{r2Overlay}, routed(),
		"the revoked recipient must not receive the sender's alerts")
}
