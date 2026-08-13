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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// usageSchema mirrors the production shape the active-user query depends on:
// users.is_banned, and overlays.created_at / last_connected_at both defaulting to
// NOW() (migration 001 + 052). The shared defaults are the whole point — a
// never-opened overlay must come out with the two timestamps equal.
const usageSchema = `
	CREATE TABLE users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		username VARCHAR(50) UNIQUE NOT NULL,
		is_banned BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE overlays (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(100) NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW(),
		last_connected_at TIMESTAMP NOT NULL DEFAULT NOW()
	);
`

func seedUser(t *testing.T, pool *pgxpool.Pool, username string, banned bool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, is_banned) VALUES ($1, $2) RETURNING id`,
		username, banned).Scan(&id)
	if err != nil {
		t.Fatalf("failed to seed user %s: %v", username, err)
	}
	return id
}

// seedOverlay inserts an overlay that was created 60 days ago and last connected
// `connectedAgo` ago (a Postgres interval literal such as '3 days').
func seedOverlay(t *testing.T, pool *pgxpool.Pool, userID, name, connectedAgo string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO overlays (user_id, name, created_at, updated_at, last_connected_at)
		VALUES ($1, $2, NOW() - INTERVAL '60 days', NOW() - INTERVAL '60 days', NOW() - $3::interval)`,
		userID, name, connectedAgo)
	if err != nil {
		t.Fatalf("failed to seed overlay %s: %v", name, err)
	}
}

// seedNeverOpenedOverlay inserts an overlay the way overlay-manager does — no
// timestamp columns given — so created_at and last_connected_at both fall to the
// statement's NOW() and are exactly equal.
func seedNeverOpenedOverlay(t *testing.T, pool *pgxpool.Pool, userID, name string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO overlays (user_id, name) VALUES ($1, $2)`, userID, name); err != nil {
		t.Fatalf("failed to seed never-opened overlay %s: %v", name, err)
	}
}

func TestActiveUserCountsRollingWindows(t *testing.T) {
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if _, err := pool.Exec(ctx, usageSchema); err != nil {
		t.Fatalf("failed to create usage schema: %v", err)
	}

	// Counts in every window: connected an hour ago.
	today := seedUser(t, pool, "today_streamer", false)
	seedOverlay(t, pool, today, "today overlay", "1 hour")

	// Counts in 7d and 30d only.
	thisWeek := seedUser(t, pool, "week_streamer", false)
	seedOverlay(t, pool, thisWeek, "week overlay", "3 days")

	// Counts in 30d only.
	thisMonth := seedUser(t, pool, "month_streamer", false)
	seedOverlay(t, pool, thisMonth, "month overlay", "20 days")

	// Churned: outside every window.
	churned := seedUser(t, pool, "churned_streamer", false)
	seedOverlay(t, pool, churned, "churned overlay", "45 days")

	// Signed up, created an overlay, never opened it: not a usage event.
	neverOpened := seedUser(t, pool, "never_opened_streamer", false)
	seedNeverOpenedOverlay(t, pool, neverOpened, "untouched overlay")

	// Banned users never count, however recently they connected.
	banned := seedUser(t, pool, "banned_streamer", true)
	seedOverlay(t, pool, banned, "banned overlay", "1 hour")

	// Multiple overlays from one streamer must collapse to a single active user.
	multi := seedUser(t, pool, "multi_overlay_streamer", false)
	seedOverlay(t, pool, multi, "multi overlay a", "2 hours")
	seedOverlay(t, pool, multi, "multi overlay b", "5 hours")

	counts, err := NewUsageRepository(pool).ActiveUserCounts(ctx)
	if err != nil {
		t.Fatalf("ActiveUserCounts() returned error: %v", err)
	}

	// today + multi
	if counts.Day != 2 {
		t.Errorf("24h window: expected 2 active users, got %d", counts.Day)
	}
	// today + multi + thisWeek
	if counts.Week != 3 {
		t.Errorf("7d window: expected 3 active users, got %d", counts.Week)
	}
	// today + multi + thisWeek + thisMonth
	if counts.Month != 4 {
		t.Errorf("30d window: expected 4 active users, got %d", counts.Month)
	}
}

func TestActiveUserCountsEmptyDatabase(t *testing.T) {
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if _, err := pool.Exec(ctx, usageSchema); err != nil {
		t.Fatalf("failed to create usage schema: %v", err)
	}

	counts, err := NewUsageRepository(pool).ActiveUserCounts(ctx)
	if err != nil {
		t.Fatalf("ActiveUserCounts() on an empty database returned error: %v", err)
	}
	if counts != (ActiveUserCounts{}) {
		t.Errorf("expected all-zero counts on an empty database, got %+v", counts)
	}
}
