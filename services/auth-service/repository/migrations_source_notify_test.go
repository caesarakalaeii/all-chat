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

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestSourceChangeNotifyIgnoresHeartbeat guards migration 059.
//
// Listeners heartbeat their active sources every ~30s via source-manager's
// ActivateSource:
//
//	UPDATE overlay_chat_sources SET is_active = true, updated_at = NOW() ...
//
// is_active is already true, so the only column that actually changes is
// updated_at. Before 059 the chat_source_changes trigger fired on EVERY update,
// so each heartbeat looked like a config change: source-manager re-fetched
// sources and re-published demand to every listener once per poll cycle, per
// overlay — observed in production as listeners flapping their channels.
//
// 059 scopes the UPDATE trigger to listener-relevant columns. This test asserts
// the new contract end-to-end against a real PostgreSQL trigger:
//   - a heartbeat-only UPDATE (updated_at) does NOT notify
//   - a real config change (channel_name) DOES notify
//   - an is_active flip (cleanup deactivation) DOES notify
//   - DELETE DOES notify
func TestSourceChangeNotifyIgnoresHeartbeat(t *testing.T) {
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()
	ctx := context.Background()

	runMigrations(t, pool, loadUpMigrations(t))

	// Seed the FK chain: users -> overlays -> overlay_chat_sources.
	var sourceID string
	err := pool.QueryRow(ctx, `
		WITH u AS (
			INSERT INTO users (twitch_id, username, display_name,
			                   access_token, refresh_token, token_expires_at)
			VALUES ('900001', 'notify_canary', 'Notify Canary',
			        'access', 'refresh', NOW() + INTERVAL '1 hour')
			RETURNING id
		), o AS (
			INSERT INTO overlays (user_id, name)
			SELECT id, 'Notify Overlay' FROM u
			RETURNING id
		)
		INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id,
		                                  channel_name, is_active)
		SELECT id, 'twitch', 'notify_chan', 'Notify Chan', true FROM o
		RETURNING id
	`).Scan(&sourceID)
	if err != nil {
		t.Fatalf("failed to seed source: %v", err)
	}

	// Dedicated connection for LISTEN — the LISTEN and every WaitForNotification
	// must share one connection, so we pin it out of the pool for the test.
	lc, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire listen connection: %v", err)
	}
	defer lc.Release()
	if _, err := lc.Exec(ctx, "LISTEN chat_source_changes"); err != nil {
		t.Fatalf("failed to LISTEN: %v", err)
	}

	// waitNotify blocks up to d for the next notification on the pinned conn.
	waitNotify := func(d time.Duration) (string, error) {
		wctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		n, err := lc.Conn().WaitForNotification(wctx)
		if err != nil {
			return "", err
		}
		return n.Payload, nil
	}

	// 1. Heartbeat write: is_active already true, only updated_at changes.
	//    MUST NOT notify.
	if _, err := pool.Exec(ctx,
		`UPDATE overlay_chat_sources SET is_active = true, updated_at = NOW() WHERE id = $1`,
		sourceID); err != nil {
		t.Fatalf("heartbeat update failed: %v", err)
	}
	if payload, err := waitNotify(2 * time.Second); err == nil {
		t.Fatalf("heartbeat-only UPDATE fired a spurious notification: %s", payload)
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error waiting for (absent) notification: %v", err)
	}

	// 2. Real config change: channel_name. MUST notify.
	if _, err := pool.Exec(ctx,
		`UPDATE overlay_chat_sources SET channel_name = 'Renamed' WHERE id = $1`,
		sourceID); err != nil {
		t.Fatalf("rename update failed: %v", err)
	}
	payload, err := waitNotify(2 * time.Second)
	if err != nil {
		t.Fatalf("config-change UPDATE did not notify: %v", err)
	}
	if !strings.Contains(payload, `"action" : "UPDATE"`) && !strings.Contains(payload, `"action":"UPDATE"`) {
		t.Errorf("expected UPDATE action in payload, got: %s", payload)
	}

	// 3. is_active flip (what the cleanup job does to stale sources). MUST notify.
	if _, err := pool.Exec(ctx,
		`UPDATE overlay_chat_sources SET is_active = false WHERE id = $1`,
		sourceID); err != nil {
		t.Fatalf("deactivate update failed: %v", err)
	}
	if _, err := waitNotify(2 * time.Second); err != nil {
		t.Fatalf("is_active flip did not notify: %v", err)
	}

	// 4. DELETE. MUST notify.
	if _, err := pool.Exec(ctx,
		`DELETE FROM overlay_chat_sources WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := waitNotify(2 * time.Second); err != nil {
		t.Fatalf("DELETE did not notify: %v", err)
	}
}
