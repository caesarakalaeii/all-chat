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

// share_status on listed sources against a Postgres with every migration applied. CI runs
// it in the test-engagement-integration job; locally point TEST_DATABASE_URL at a migrated
// database and run: go test -tags=integration ./repository/
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

// A shared_overlay source carries the status of the share that feeds it, so the editor
// can badge a revoked or expired one. The source stores the sender overlay id, not the
// share id, so the old share-id join never matched and share_status was always empty.
func TestListByOverlayIDForUser_SharedSourceStatus(t *testing.T) {
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

	tag := fmt.Sprintf("om_share_%d", time.Now().UnixNano())
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
			`INSERT INTO overlays (user_id, name, is_public_for_viewers) VALUES ($1, 'share test', false) RETURNING id`,
			userID).Scan(&id))
		overlayIDs = append(overlayIDs, id)
		return id
	}
	share := func(sender, senderOverlay, recipient, status string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, responded_at)
			VALUES ($1, $2, $3, $4, NOW()) RETURNING id`, sender, senderOverlay, recipient, status).Scan(&id))
		return id
	}

	sender, other, recipient := user("s"), user("o"), user("r")
	senderOverlay := overlay(sender)
	recipientOverlay := overlay(recipient)
	_, err = pool.Exec(ctx, `
		INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name, is_active)
		VALUES ($1, 'shared_overlay', $2, 'share test', false)`, recipientOverlay, senderOverlay)
	require.NoError(t, err)

	repo := &SourceRepository{pool: pool}
	status := func() string {
		sources, err := repo.ListByOverlayIDForUser(ctx, recipientOverlay, recipient)
		require.NoError(t, err)
		require.Len(t, sources, 1)
		if sources[0].ShareStatus == nil {
			return ""
		}
		return *sources[0].ShareStatus
	}

	mine := share(sender, senderOverlay, recipient, "accepted")
	assert.Equal(t, "accepted", status())

	_, err = pool.Exec(ctx, `UPDATE share_requests SET status = 'revoked' WHERE id = $1`, mine)
	require.NoError(t, err)
	share(sender, senderOverlay, other, "accepted") // someone else's share is not this source's
	assert.Equal(t, "revoked", status())

	share(sender, senderOverlay, recipient, "accepted") // shared again
	assert.Equal(t, "accepted", status())
}
