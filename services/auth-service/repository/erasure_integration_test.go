//go:build integration

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

// Account and viewer erasure (ADR-0088) against a real Postgres that has the full
// migration set applied, exactly as scripts/run-migrations.sh leaves a cluster. CI's
// integration job applies the migrations and runs these; locally:
//
//	TEST_DATABASE_URL=postgres://user@localhost:5432/db go test -tags=integration -run Erase ./repository/

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func erasureDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://allchat:allchat_dev_password@localhost:5432/allchat"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("requires DB: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("requires DB: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func eraseSfx() string { return uuid.NewString()[:8] }

func eraseSeedUser(t *testing.T, pool *pgxpool.Pool, twitchID string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $1, $1, 'x', 'x', NOW() + INTERVAL '1 day') RETURNING id`, twitchID).Scan(&id))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	return id
}

func eraseSeedOverlay(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO overlays (user_id, name) VALUES ($1, 'erasure test') RETURNING id`, userID).Scan(&id))
	return id
}

// eraseSeedChatter writes a viewer exactly as engagement-service's GetOrCreateViewerByPlatform
// does for someone who votes, bets or earns points by chat: a viewers row and its platform
// identity, no session, no cosmetics row.
func eraseSeedChatter(t *testing.T, pool *pgxpool.Pool, platform, platformUserID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO viewers DEFAULT VALUES RETURNING id`).Scan(&id))
	_, err := pool.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, $2, $3)`,
		id, platform, platformUserID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM viewers WHERE id = $1`, id) })
	return id
}

func eraseSeedSession(t *testing.T, pool *pgxpool.Pool, platform, platformUserID string, viewerID, userID *uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name, access_token,
		                              token_expires_at, viewer_id, user_id)
		 VALUES ($1, $2, $2, $2, 'enc', NOW() + INTERVAL '1 day', $3, $4) RETURNING id`,
		platform, platformUserID, viewerID, userID).Scan(&id))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM viewer_sessions WHERE id = $1`, id) })
	return id
}

// engagement is one streamer's points economy with an open poll and an open prediction.
type engagement struct {
	overlay, poll, option, prediction, outcome uuid.UUID
}

func eraseSeedEngagement(t *testing.T, pool *pgxpool.Pool, overlayID uuid.UUID) engagement {
	t.Helper()
	ctx := context.Background()
	e := engagement{overlay: overlayID}
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO polls (overlay_id, question) VALUES ($1, 'q') RETURNING id`, overlayID).Scan(&e.poll))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO poll_options (poll_id, idx, label) VALUES ($1, 1, 'a') RETURNING id`, e.poll).Scan(&e.option))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO predictions (overlay_id, title, state) VALUES ($1, 'p', 'ACTIVE') RETURNING id`, overlayID).Scan(&e.prediction))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO prediction_outcomes (prediction_id, idx, label) VALUES ($1, 1, 'yes') RETURNING id`, e.prediction).Scan(&e.outcome))
	return e
}

// eraseParticipate gives the viewer a balance, a ledger row, a vote and a wager in e.
func eraseParticipate(t *testing.T, pool *pgxpool.Pool, e engagement, viewerID uuid.UUID, wager int64) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO viewer_points (viewer_id, overlay_id, balance) VALUES ($1, $2, 100)`, []any{viewerID, e.overlay}},
		{`INSERT INTO points_transactions (viewer_id, overlay_id, delta, reason, dedup_key) VALUES ($1, $2, 100, 'earn_chat', $3)`,
			[]any{viewerID, e.overlay, "erasure-" + uuid.NewString()}},
		{`INSERT INTO poll_votes (poll_id, viewer_id, option_id) VALUES ($1, $2, $3)`, []any{e.poll, viewerID, e.option}},
		{`INSERT INTO prediction_entries (prediction_id, viewer_id, outcome_id, amount) VALUES ($1, $2, $3, $4)`,
			[]any{e.prediction, viewerID, e.outcome, wager}},
	} {
		_, err := pool.Exec(ctx, q.sql, q.args...)
		require.NoError(t, err, q.sql)
	}
}

func eraseCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), sql, args...).Scan(&n), sql)
	return n
}

// eraseViewerRows counts every row a viewer owns, across all the tables viewer erasure covers.
func eraseViewerRows(t *testing.T, pool *pgxpool.Pool, viewerID uuid.UUID) int {
	t.Helper()
	return eraseCount(t, pool, `SELECT
		(SELECT COUNT(*) FROM viewers WHERE id = $1) +
		(SELECT COUNT(*) FROM viewer_platform_identities WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM viewer_cosmetics WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM viewer_sessions WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM viewer_points WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM points_transactions WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM poll_votes WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM prediction_entries WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM patreon_oauth_tokens WHERE viewer_id = $1) +
		(SELECT COUNT(*) FROM premium_subscriptions WHERE viewer_id = $1)`, viewerID)
}

func TestEraseUser_WithShareRequests(t *testing.T) {
	pool := erasureDB(t)
	ctx := context.Background()
	repo := NewUserRepository(pool, nil)

	setup := func(t *testing.T) (sender, recipient, senderOverlay, recipientOverlay, share uuid.UUID) {
		sender, recipient = eraseSeedUser(t, pool, "share-s-"+eraseSfx()), eraseSeedUser(t, pool, "share-r-"+eraseSfx())
		senderOverlay, recipientOverlay = eraseSeedOverlay(t, pool, sender), eraseSeedOverlay(t, pool, recipient)
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, recipient_overlay_id, status)
			 VALUES ($1, $2, $3, $4, 'accepted') RETURNING id`,
			sender, senderOverlay, recipient, recipientOverlay).Scan(&share))
		// The recipient subscribed to the sender's overlay (overlay-manager stores the
		// sender overlay id as the shared_overlay source's channel_id).
		_, err := pool.Exec(ctx,
			`INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name)
			 VALUES ($1, 'shared_overlay', $2, 'shared')`, recipientOverlay, senderOverlay.String())
		require.NoError(t, err)
		return
	}

	t.Run("sender", func(t *testing.T) {
		sender, recipient, _, recipientOverlay, share := setup(t)
		_, err := repo.Delete(ctx, sender.String())
		require.NoError(t, err, "a streamer who ever sent a share request must be able to delete their account")
		assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM share_requests WHERE id = $1`, share))
		assert.Zero(t, eraseCount(t, pool,
			`SELECT COUNT(*) FROM overlay_chat_sources WHERE overlay_id = $1 AND platform = 'shared_overlay'`, recipientOverlay),
			"the recipient's source for the deleted overlay must not dangle")
		assert.Equal(t, 1, eraseCount(t, pool, `SELECT COUNT(*) FROM users WHERE id = $1`, recipient))
	})

	t.Run("recipient", func(t *testing.T) {
		sender, recipient, senderOverlay, _, share := setup(t)
		_, err := repo.Delete(ctx, recipient.String())
		require.NoError(t, err, "a streamer who ever received a share request must be able to delete their account")
		assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM share_requests WHERE id = $1`, share))
		assert.Equal(t, 1, eraseCount(t, pool, `SELECT COUNT(*) FROM overlays WHERE id = $1 AND user_id = $2`, senderOverlay, sender))
	})

	t.Run("shared overlay alone", func(t *testing.T) {
		_, _, senderOverlay, recipientOverlay, share := setup(t)
		_, err := pool.Exec(ctx, `DELETE FROM overlays WHERE id = $1`, senderOverlay)
		require.NoError(t, err, "an overlay that was ever shared must be deletable")
		assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM share_requests WHERE id = $1`, share))
		assert.Zero(t, eraseCount(t, pool,
			`SELECT COUNT(*) FROM overlay_chat_sources WHERE overlay_id = $1 AND platform = 'shared_overlay'`, recipientOverlay))
	})
}

func TestEraseUser_AdminWhoBannedOthers(t *testing.T) {
	pool := erasureDB(t)
	ctx := context.Background()
	admin, banned := eraseSeedUser(t, pool, "ban-admin-"+eraseSfx()), eraseSeedUser(t, pool, "ban-user-"+eraseSfx())
	_, err := pool.Exec(ctx, `UPDATE users SET is_banned = TRUE, banned_by = $1 WHERE id = $2`, admin, banned)
	require.NoError(t, err)
	platformID := "ban-pid-" + eraseSfx()
	_, err = pool.Exec(ctx,
		`INSERT INTO banned_platform_ids (platform, platform_id, banned_by, reason) VALUES ('twitch', $1, $2, 'spam')`,
		platformID, admin)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM banned_platform_ids WHERE platform_id = $1`, platformID) })

	_, err = NewUserRepository(pool, nil).Delete(ctx, admin.String())
	require.NoError(t, err, "an admin who ever banned someone must be able to delete their account")

	// The bans outlive the admin who issued them.
	assert.Equal(t, 1, eraseCount(t, pool, `SELECT COUNT(*) FROM users WHERE id = $1 AND is_banned AND banned_by IS NULL`, banned))
	assert.Equal(t, 1, eraseCount(t, pool,
		`SELECT COUNT(*) FROM banned_platform_ids WHERE platform_id = $1 AND is_active AND banned_by IS NULL`, platformID))
}

func TestEraseUser_ErasesOwnViewerIdentityAndOverlayHistory(t *testing.T) {
	pool := erasureDB(t)
	ctx := context.Background()

	twitchID := "own-tw-" + eraseSfx()
	user := eraseSeedUser(t, pool, twitchID)
	userOverlay := eraseSeedOverlay(t, pool, user)
	other := eraseSeedUser(t, pool, "own-other-"+eraseSfx())
	e := eraseSeedEngagement(t, pool, eraseSeedOverlay(t, pool, other))

	// The streamer's own viewer identity: Twitch (same account as the streamer login,
	// linked by the viewer callback) plus a YouTube channel they linked to it.
	ytID := "own-yt-" + eraseSfx()
	self := eraseSeedChatter(t, pool, "twitch", twitchID)
	_, err := pool.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, 'youtube', $2)`, self, ytID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO viewer_cosmetics (viewer_id, name_color) VALUES ($1, '#ff0000')`, self)
	require.NoError(t, err)
	twSession := eraseSeedSession(t, pool, "twitch", twitchID, &self, &user)
	eraseSeedSession(t, pool, "youtube", ytID, &self, nil)
	eraseParticipate(t, pool, e, self, 40)
	_, err = pool.Exec(ctx,
		`INSERT INTO viewer_message_history (viewer_session_id, streamer_user_id, platform, channel_id, channel_name, message_text)
		 VALUES ($1, $2, 'twitch', 'c', 'c', 'hello')`, twSession, other)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO patreon_oauth_tokens (viewer_id, patreon_user_id, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $2, 'enc', 'enc', NOW() + INTERVAL '1 day')`, self, "own-patreon-"+eraseSfx())
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO premium_subscriptions (viewer_id, provider_user_id, status, product) VALUES ($1, 'p', 'active', 'viewer')`, self)
	require.NoError(t, err)

	// Someone else in the same economy: must come through untouched.
	bystander := eraseSeedChatter(t, pool, "twitch", "own-bystander-"+eraseSfx())
	eraseParticipate(t, pool, e, bystander, 25)
	bystanderRows := eraseViewerRows(t, pool, bystander)

	// Rows keyed to the streamer's overlay with no foreign key.
	_, err = pool.Exec(ctx,
		`INSERT INTO alert_events (id, overlay_id, platform, event_type, user_name, occurred_at)
		 VALUES ($1, $2, 'twitch', 'follow', 'someone', NOW())`, uuid.New(), userOverlay)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO youtube_quota_audit_log (date, operation_type, service_name, endpoint, units_delta, units_before, units_after, overlay_id)
		 VALUES (CURRENT_DATE, 'consume', 'test', 'search', 1, 0, 1, $1)`, userOverlay)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO youtube_channel_quota (channel_id, user_id) VALUES ($1, $2)`, ytID, user)
	require.NoError(t, err)
	// The moderation audit log is kept on purpose (ADR-0017 forensics).
	modAction := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO moderation_actions (id, overlay_id, actor_user_id, platform, channel_id, action, outcome)
		 VALUES ($1, $2, $3, 'twitch', 'c', 'delete', 'success')`, modAction, userOverlay, user)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM moderation_actions WHERE id = $1`, modAction) })

	erased, err := NewUserRepository(pool, nil).Delete(ctx, user.String())
	require.NoError(t, err)

	assert.ElementsMatch(t, []LinkedPlatform{{"twitch", twitchID}, {"youtube", ytID}}, erased)
	assert.Zero(t, eraseViewerRows(t, pool, self), "the streamer's own viewer data must be erased with the account")
	assert.Zero(t, eraseCount(t, pool,
		`SELECT COUNT(*) FROM viewer_sessions WHERE platform_user_id = ANY($1)`, []string{twitchID, ytID}))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM viewer_message_history WHERE viewer_session_id = $1`, twSession))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM alert_events WHERE overlay_id = $1`, userOverlay))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM youtube_quota_audit_log WHERE overlay_id = $1`, userOverlay))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM youtube_channel_quota WHERE user_id = $1`, user))
	assert.Equal(t, 1, eraseCount(t, pool, `SELECT COUNT(*) FROM moderation_actions WHERE id = $1`, modAction))

	assert.Equal(t, bystanderRows, eraseViewerRows(t, pool, bystander), "another viewer's data must be untouched")
	assert.Equal(t, 25, eraseCount(t, pool,
		`SELECT COALESCE(SUM(amount), 0) FROM prediction_entries WHERE prediction_id = $1`, e.prediction),
		"the open prediction's pool holds exactly the remaining wagers")
}

// A chatter who only ever voted, bet or earned points by chat command has no session:
// engagement-service created the viewer on the fly. Signing in as a viewer with the
// same platform account must resolve to those rows, and deleting must erase them.
func TestEraseViewer_ChatterCreatedWithoutLogin(t *testing.T) {
	pool := erasureDB(t)
	ctx := context.Background()
	identities := NewViewerIdentityRepository(pool, nil)

	e := eraseSeedEngagement(t, pool, eraseSeedOverlay(t, pool, eraseSeedUser(t, pool, "chat-streamer-"+eraseSfx())))
	kickID := "chat-kick-" + eraseSfx()
	chatter := eraseSeedChatter(t, pool, "kick", kickID)
	eraseParticipate(t, pool, e, chatter, 30)
	bystander := eraseSeedChatter(t, pool, "kick", "chat-bystander-"+eraseSfx())
	eraseParticipate(t, pool, e, bystander, 15)
	bystanderRows := eraseViewerRows(t, pool, bystander)

	// Viewer login: the auth-service resolver finds the engagement-created viewer.
	resolved, err := identities.GetOrCreateViewerByPlatform(ctx, "kick", kickID)
	require.NoError(t, err)
	require.Equal(t, chatter, resolved, "viewer login must resolve to the rows the chat created")
	eraseSeedSession(t, pool, "kick", kickID, &resolved, nil)

	erased, err := identities.EraseViewer(ctx, resolved, "kick", kickID)
	require.NoError(t, err)

	assert.Equal(t, []LinkedPlatform{{"kick", kickID}}, erased)
	assert.Zero(t, eraseViewerRows(t, pool, chatter))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM viewer_sessions WHERE platform = 'kick' AND platform_user_id = $1`, kickID))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM viewer_platform_identities WHERE platform_user_id = $1`, kickID))
	assert.Equal(t, bystanderRows, eraseViewerRows(t, pool, bystander))
	assert.Equal(t, 15, eraseCount(t, pool,
		`SELECT COALESCE(SUM(amount), 0) FROM prediction_entries WHERE prediction_id = $1`, e.prediction))
	assert.Equal(t, 1, eraseCount(t, pool, `SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1`, e.poll))
}

// A viewer token issued before the durable viewer identity existed carries no viewer_id;
// the platform identity in the token still finds every linked platform and session.
func TestEraseViewer_TokenWithoutViewerID(t *testing.T) {
	pool := erasureDB(t)
	ctx := context.Background()
	twID, ytID := "legacy-tw-"+eraseSfx(), "legacy-yt-"+eraseSfx()
	viewer := eraseSeedChatter(t, pool, "twitch", twID)
	_, err := pool.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, 'youtube', $2)`, viewer, ytID)
	require.NoError(t, err)
	eraseSeedSession(t, pool, "twitch", twID, nil, nil)
	eraseSeedSession(t, pool, "youtube", ytID, &viewer, nil)

	erased, err := NewViewerIdentityRepository(pool, nil).EraseViewer(ctx, uuid.Nil, "twitch", twID)
	require.NoError(t, err)

	assert.ElementsMatch(t, []LinkedPlatform{{"twitch", twID}, {"youtube", ytID}}, erased)
	assert.Zero(t, eraseViewerRows(t, pool, viewer))
	assert.Zero(t, eraseCount(t, pool, `SELECT COUNT(*) FROM viewer_sessions WHERE platform_user_id = ANY($1)`, []string{twID, ytID}))
}

// Migration 119 is re-applied on every pod start (scripts/run-migrations.sh): re-running
// it must succeed and leave the same delete rules in place.
func TestEraseMigration119_Rerunnable(t *testing.T) {
	pool := erasureDB(t)
	ctx := context.Background()
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "119_*.sql"))
	require.NoError(t, err)
	var up string
	for _, f := range files {
		if !strings.HasSuffix(f, "_down.sql") {
			up = f
		}
	}
	require.NotEmpty(t, up, "migration 119 not found")
	sql, err := os.ReadFile(up)
	require.NoError(t, err)
	for range 2 {
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err)
	}

	rules := map[string]string{}
	rows, err := pool.Query(ctx, `
		SELECT conrelid::regclass::text || '.' || a.attname, confdeltype::text
		FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
		WHERE c.contype = 'f' AND c.conrelid::regclass::text IN ('share_requests', 'users', 'banned_platform_ids')
		  AND c.confrelid::regclass::text IN ('users', 'overlays')`)
	require.NoError(t, err)
	for rows.Next() {
		var col, rule string
		require.NoError(t, rows.Scan(&col, &rule))
		rules[col] += rule
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, map[string]string{
		"share_requests.sender_user_id":       "c",
		"share_requests.recipient_user_id":    "c",
		"share_requests.sender_overlay_id":    "c",
		"share_requests.recipient_overlay_id": "n",
		"users.banned_by":                     "n",
		"banned_platform_ids.banned_by":       "n",
	}, rules, "exactly one foreign key per column, with the erasure-safe delete rule")
}
