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

// The share fan-out of FindOverlaysForMessage against a Postgres with every migration
// applied. CI runs it in the test-engagement-integration job; locally point
// TEST_DATABASE_URL at a migrated database and run: go test -tags=integration ./router/
package router

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A revoked recipient must stop receiving the sender's chat even while the same overlay
// stays shared with someone else. The fan-out used to join any accepted share of the
// sender overlay, whoever its recipient was, so one live share kept every past recipient
// subscribed. The recipient's source is left active on purpose: is_active follows the
// overlay's websocket connection, so the share itself has to gate the route.
func TestFindOverlaysForMessage_RevokedRecipientStopsReceiving(t *testing.T) {
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

	tag := fmt.Sprintf("mp_share_%d", time.Now().UnixNano())
	user := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
			VALUES ($1, $1, $1, 'a', 'r', NOW() + INTERVAL '1 hour') RETURNING id`, tag+name).Scan(&id))
		userIDs = append(userIDs, id)
		return id
	}
	overlay := func(userID, platform, channelID string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO overlays (user_id, name) VALUES ($1, 'share test') RETURNING id`, userID).Scan(&id))
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
	senderOverlay := overlay(sender, "twitch", tag)
	r1Overlay := overlay(r1, "shared_overlay", senderOverlay)
	r2Overlay := overlay(r2, "shared_overlay", senderOverlay)
	share1 := share(sender, senderOverlay, r1)
	share(sender, senderOverlay, r2)

	r := NewRouter(NewRepository(pool), zap.NewNop())
	routed := func() []string {
		targets, err := r.Route(ctx, "twitch", tag)
		require.NoError(t, err)
		ids := make([]string, 0, len(targets))
		for _, target := range targets {
			ids = append(ids, target.OverlayID)
		}
		return ids
	}

	assert.ElementsMatch(t, []string{senderOverlay, r1Overlay, r2Overlay}, routed())

	_, err = pool.Exec(ctx, `UPDATE share_requests SET status = 'revoked' WHERE id = $1`, share1)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{senderOverlay, r2Overlay}, routed(),
		"the revoked recipient must not receive the sender's chat")
}
