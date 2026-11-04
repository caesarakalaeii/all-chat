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

// End-to-end tests for the retention sweeper (ADR-0089). They call the binary's
// run() against a Postgres with every migration applied, so each rule is checked
// against the real schema, not a copy of it. CI runs them in the
// test-engagement-integration job; locally point TEST_DATABASE_URL at a migrated
// database and run: go test -tags=integration ./cmd/retention-sweeper/
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://allchat:allchat_dev_password@localhost:5432/allchat?sslmode=disable"
}

// row is one seeded row and whether the sweep must delete it.
type row struct {
	name        string
	table       string
	id          string
	wantDeleted bool
}

type fixture struct {
	t    *testing.T
	conn *pgx.Conn
	rows []row
	// parents are the users and viewer sessions the rows hang off; deleting them
	// cascades to everything else the test left behind.
	parents []row
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN())
	require.NoError(t, err, "integration tests need a migrated Postgres (TEST_DATABASE_URL)")
	f := &fixture{t: t, conn: conn}
	t.Cleanup(func() {
		for _, r := range append(f.rows, f.parents...) {
			_, _ = conn.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id::text = $1`, r.table), r.id)
		}
		conn.Close(ctx)
	})
	return f
}

// insert runs an INSERT ... RETURNING id and returns the id as text.
func (f *fixture) insert(sql string, args ...any) string {
	f.t.Helper()
	var id string
	require.NoError(f.t, f.conn.QueryRow(context.Background(), sql, args...).Scan(&id), sql)
	return id
}

// seed inserts a row and records whether the sweep must delete it.
func (f *fixture) seed(name, table string, wantDeleted bool, sql string, args ...any) {
	f.t.Helper()
	f.rows = append(f.rows, row{name: name, table: table, id: f.insert(sql, args...), wantDeleted: wantDeleted})
}

func (f *fixture) exists(table, id string) bool {
	f.t.Helper()
	var ok bool
	require.NoError(f.t, f.conn.QueryRow(context.Background(),
		fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE id::text = $1)`, table), id).Scan(&ok))
	return ok
}

func (f *fixture) assertSwept() {
	f.t.Helper()
	for _, r := range f.rows {
		if r.wantDeleted {
			assert.False(f.t, f.exists(r.table, r.id), "%s: %s row should have been deleted", r.name, r.table)
		} else {
			assert.True(f.t, f.exists(r.table, r.id), "%s: %s row must be kept", r.name, r.table)
		}
	}
}

// owner creates a streamer account with one overlay and returns both ids.
func (f *fixture) owner() (userID, overlayID string) {
	userID = f.insert(`
		INSERT INTO users (twitch_id, auth_provider, username, display_name, access_token, refresh_token, token_expires_at)
		VALUES (gen_random_uuid()::text, 'twitch', 'rs_' || gen_random_uuid(), 'Retention Sweeper', 'a', 'r', NOW() + INTERVAL '1 hour')
		RETURNING id::text`)
	f.parents = append(f.parents, row{table: "users", id: userID})
	overlayID = f.insert(`INSERT INTO overlays (user_id, name) VALUES ($1, 'rs') RETURNING id::text`, userID)
	return userID, overlayID
}

func (f *fixture) viewerSession() string {
	id := f.insert(`
		INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name, access_token, token_expires_at)
		VALUES ('twitch', gen_random_uuid()::text, 'rs_viewer', 'RS Viewer', 'a', NOW() + INTERVAL '1 hour')
		RETURNING id::text`)
	f.parents = append(f.parents, row{table: "viewer_sessions", id: id})
	return id
}

// sweepEnv is the environment the CronJob gives the binary.
func sweepEnv(key string) string {
	if key == "DATABASE_URL" {
		return testDSN()
	}
	return ""
}

// seedLogs seeds the time-boxed logs with one row just past and one just inside
// each retention window.
func seedLogs(f *fixture) {
	userID, overlayID := f.owner()
	sessionID := f.viewerSession()

	const msg = `
		INSERT INTO viewer_message_history
		       (viewer_session_id, streamer_user_id, platform, channel_id, channel_name, message_text, created_at)
		VALUES ($1, $2, 'twitch', 'c', 'c', 'hello', NOW() - $3::interval)
		RETURNING id::text`
	f.seed("message older than 1 hour", "viewer_message_history", true, msg, sessionID, userID, "61 minutes")
	f.seed("message within 1 hour", "viewer_message_history", false, msg, sessionID, userID, "59 minutes")

	const quota = `
		INSERT INTO youtube_quota_audit_log
		       (date, "timestamp", operation_type, service_name, endpoint, units_delta, units_before, units_after)
		VALUES (CURRENT_DATE, NOW() - $1::interval, 'record', 'retention-test', 'Videos.List', 1, 0, 1)
		RETURNING id::text`
	f.seed("quota audit older than 30 days", "youtube_quota_audit_log", true, quota, "30 days 1 hour")
	f.seed("quota audit within 30 days", "youtube_quota_audit_log", false, quota, "29 days 23 hours")

	const alert = `
		INSERT INTO alert_events (id, overlay_id, platform, event_type, occurred_at, created_at)
		VALUES (gen_random_uuid(), $1, 'twitch', 'follow', NOW() - $2::interval, NOW() - $2::interval)
		RETURNING id::text`
	f.seed("alert event older than 30 days", "alert_events", true, alert, overlayID, "30 days 1 hour")
	f.seed("alert event within 30 days", "alert_events", false, alert, overlayID, "29 days 23 hours")

	const mod = `
		INSERT INTO moderation_actions (overlay_id, actor_user_id, platform, channel_id, action, outcome, created_at)
		VALUES ($1, $2, 'twitch', 'c', 'delete', 'success', NOW() - $3::interval)
		RETURNING id::text`
	f.seed("moderation action older than 90 days", "moderation_actions", true, mod, overlayID, userID, "90 days 1 hour")
	f.seed("moderation action within 90 days", "moderation_actions", false, mod, overlayID, userID, "89 days 23 hours")

	const link = `
		INSERT INTO device_link_requests (flow, pkce_challenge, device_name, expires_at, device_token_id)
		VALUES ('code', 'challenge', 'Stream Deck', NOW() - $1::interval, $2)
		RETURNING id::text`
	deviceTokenID := f.insert(`
		INSERT INTO device_tokens (user_id, overlay_id, name, token_hash)
		VALUES ($1, $2, 'Stream Deck', sha256(gen_random_uuid()::text::bytea))
		RETURNING id::text`, userID, overlayID)
	f.seed("abandoned link request expired over 1 hour ago", "device_link_requests", true, link, "61 minutes", nil)
	f.seed("abandoned link request inside the 1 hour grace", "device_link_requests", false, link, "59 minutes", nil)
	f.seed("link request that minted a device token", "device_link_requests", false, link, "30 days", deviceTokenID)
}

// seedCredentials seeds OAuth credentials: dead ones the sweep must delete, and
// ones that look expired but can still be refreshed or are still needed, which it
// must never touch (ADR-0089, "dead" per table).
func seedCredentials(f *fixture) {
	userID, _ := f.owner()
	sessionID := f.viewerSession()

	// TikTok (legacy): nothing in All-Chat reads, writes or refreshes these rows,
	// so the access token's expiry is the moment the credential died.
	const tiktok = `
		INSERT INTO tiktok_oauth_tokens (user_id, open_id, access_token, refresh_token, expiry)
		VALUES ($1, gen_random_uuid()::text, 'a', 'r', NOW() - $2::interval)
		RETURNING id::text`
	f.seed("tiktok token expired over 7 days ago", "tiktok_oauth_tokens", true, tiktok, userID, "7 days 1 hour")
	f.seed("tiktok token expired under 7 days ago", "tiktok_oauth_tokens", false, tiktok, userID, "6 days 23 hours")

	// Kick: only rows with a numeric kick_user_id are ever refreshed (moderation,
	// chat send); a legacy listener row without one is dead once expired.
	const kick = `
		INSERT INTO kick_oauth_tokens (user_id, channel_id, kick_user_id, access_token, refresh_token, expiry)
		VALUES ($1, 'rs_' || gen_random_uuid(), $2, 'a', 'r', NOW() - $3::interval)
		RETURNING id::text`
	f.seed("legacy kick row expired over 7 days ago", "kick_oauth_tokens", true, kick, userID, nil, "7 days 1 hour")
	f.seed("legacy kick row expired under 7 days ago", "kick_oauth_tokens", false, kick, userID, nil, "6 days 23 hours")
	f.seed("linked kick credential, refreshable on use", "kick_oauth_tokens", false, kick, userID, "4242", "60 days")

	// Instagram: no refresh grant exists, so an expired token is dead. NULL
	// expires_at is a non-expiring Page-derived token.
	const instagram = `
		INSERT INTO instagram_oauth_tokens (user_id, ig_user_id, access_token, expires_at)
		VALUES ($1, gen_random_uuid()::text, 'a', NOW() - $2::interval)
		RETURNING id::text`
	f.seed("instagram token expired over 7 days ago", "instagram_oauth_tokens", true, instagram, userID, "7 days 1 hour")
	f.seed("instagram token expired under 7 days ago", "instagram_oauth_tokens", false, instagram, userID, "6 days 23 hours")
	f.seed("non-expiring instagram token", "instagram_oauth_tokens", false, instagram, userID, nil)

	// The tables the old cleanup_expired_oauth_tokens() deleted from by access-token
	// expiry alone. Each of these can still be refreshed (or is still needed), so a
	// month-old expiry must not get them deleted.
	f.seed("youtube channel token with an old expiry", "youtube_oauth_tokens", false, `
		INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry)
		VALUES ($1, 'UC' || gen_random_uuid(), 'a', '', NOW() - INTERVAL '30 days')
		RETURNING id::text`, userID)
	f.seed("linked twitch token with an old expiry", "twitch_oauth_tokens", false, `
		INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at)
		VALUES ($1, gen_random_uuid()::text, 'rs_' || gen_random_uuid(), 'a', 'r', NOW() - INTERVAL '30 days')
		RETURNING id::text`, userID)
	f.seed("patreon token with an old expiry", "patreon_oauth_tokens", false, `
		INSERT INTO patreon_oauth_tokens (user_id, patreon_user_id, access_token, refresh_token, token_expires_at)
		VALUES ($1, gen_random_uuid()::text, 'a', 'r', NOW() - INTERVAL '30 days')
		RETURNING id::text`, userID)
	f.seed("moderator credential without a refresh token", "mod_oauth_credentials", false, `
		INSERT INTO mod_oauth_credentials (user_id, platform, platform_user_id, access_token, token_expires_at)
		VALUES ($1, 'twitch', gen_random_uuid()::text, 'a', NOW() - INTERVAL '30 days')
		RETURNING id::text`, userID)
	f.seed("bot account with an old expiry", "chat_bot_accounts", false, `
		INSERT INTO chat_bot_accounts (owner_user_id, platform, platform_user_id, login, display_name, token_expires_at)
		VALUES ($1, 'twitch', gen_random_uuid()::text, 'rs_bot', 'RS Bot', NOW() - INTERVAL '30 days')
		RETURNING id::text`, userID)
	f.rows = append(f.rows, row{name: "viewer session with an old expiry", table: "viewer_sessions", id: sessionID})
	_, err := f.conn.Exec(context.Background(),
		`UPDATE viewer_sessions SET token_expires_at = NOW() - INTERVAL '30 days' WHERE id::text = $1`, sessionID)
	require.NoError(f.t, err)
}

func TestRun_DeletesRowsPastRetention(t *testing.T) {
	f := newFixture(t)
	seedLogs(f)
	seedCredentials(f)

	// A batch size of 1 forces the batching loop through several batches per table.
	code := run(context.Background(), []string{"--batch-size=1", "--batch-delay-ms=0"}, sweepEnv)
	require.Equal(t, 0, code)
	f.assertSwept()
}

func TestRun_IdempotentAndSafeConcurrently(t *testing.T) {
	f := newFixture(t)
	seedLogs(f)
	seedCredentials(f)

	// Two overlapping sweeps (a manual Job next to the CronJob) must both succeed
	// and leave the same result as one.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = run(context.Background(), []string{"--batch-size=1", "--batch-delay-ms=0"}, sweepEnv)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, []int{0, 0}, codes)
	f.assertSwept()

	// A third run over already-swept data is a no-op.
	require.Equal(t, 0, run(context.Background(), nil, sweepEnv))
	f.assertSwept()
}

func TestRun_RequiresDatabaseURL(t *testing.T) {
	assert.NotEqual(t, 0, run(context.Background(), nil, func(string) string { return "" }))
}

func TestMigration118_DropsDeadPgCronFunctions(t *testing.T) {
	f := newFixture(t)
	var names []string
	rows, err := f.conn.Query(context.Background(), `
		SELECT proname FROM pg_proc
		 WHERE proname IN ('cleanup_old_message_history', 'cleanup_expired_oauth_tokens',
		                   'cleanup_old_youtube_audit_logs')`)
	require.NoError(t, err)
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	assert.Empty(t, names, "the pg_cron-only cleanup functions are superseded by the sweeper and must be dropped")
}
