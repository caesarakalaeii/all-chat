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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// treasureChestMigration is the file that adds enable_tiktok_treasure_chests.
const treasureChestMigration = "084_tiktok_treasure_chest_event_setting.sql"

// TestEventSettingsTreasureChestBackfillsExistingOverlays applies the migration
// set in two passes with an overlay created in between, so 084 runs against a
// POPULATED overlay_event_settings table — exactly what a production deploy
// does. A NOT NULL column added without a default would fail outright there,
// and a nullable one would make the 27-column Scan blow up on every existing
// overlay, so this asserts existing rows come out of the migration with coin
// chests enabled.
func TestEventSettingsTreasureChestBackfillsExistingOverlays(t *testing.T) {
	pool, cleanup := setupEventSettingsTestDB(t)
	defer cleanup()

	before, fromChest := splitAtTreasureChestMigration(t, loadUpMigrations(t))
	runMigrations(t, pool, before)

	// An overlay that predates the column, i.e. every streamer already on prod.
	overlayID := seedOverlayWithEventSettings(t, pool, "chest_backfill_canary")

	runMigrations(t, pool, fromChest)

	repo := NewEventSettingsRepositoryFromPool(pool)
	settings, err := repo.GetByOverlayID(context.Background(), overlayID)
	require.NoError(t, err)
	require.NotNil(t, settings)

	assert.True(t, settings.EnableTikTokTreasureChests,
		"existing overlays must come out of migration 084 with coin chests enabled — the events never reached an overlay before, so enabling them is the fix")
}

// TestEventSettingsUpdateRoundTripKeepsEveryTogglePositioned is the guard for
// the placeholder renumbering that inserting enable_tiktok_treasure_chests into
// the middle of the UPDATE required: a shifted $N binds a value to a
// NEIGHBOURING column, which silently flips an unrelated event toggle for the
// streamer. The written pattern alternates on/off so any single-position shift
// changes the round-tripped value and fails the comparison.
func TestEventSettingsUpdateRoundTripKeepsEveryTogglePositioned(t *testing.T) {
	pool, cleanup := setupEventSettingsTestDB(t)
	defer cleanup()

	runMigrations(t, pool, loadUpMigrations(t))
	overlayID := seedOverlayWithEventSettings(t, pool, "chest_roundtrip_canary")

	ctx := context.Background()
	repo := NewEventSettingsRepositoryFromPool(pool)

	settings, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.NotNil(t, settings)

	settings.EnableTwitchSubs = true
	settings.EnableTwitchResubs = false
	settings.EnableTwitchGiftSubs = true
	settings.EnableTwitchBits = false
	settings.EnableTwitchRaids = true
	settings.EnableTwitchChannelPoints = false
	settings.EnableTwitchFollows = true
	settings.EnableTwitchWatchStreaks = false
	settings.EnableYouTubeSuperChat = true
	settings.EnableYouTubeSuperSticker = false
	settings.EnableYouTubeMembers = true
	settings.EnableYouTubeMemberMilestones = false
	settings.EnableYouTubeMemberGifts = true
	settings.EnableKickSubs = false
	settings.EnableKickGifts = true
	settings.EnableTikTokLikes = false
	settings.EnableTikTokGifts = true
	settings.EnableTikTokFollows = false
	settings.EnableTikTokShares = true
	settings.EnableTikTokTreasureChests = false
	settings.EnableTokenWarnings = true
	settings.TikTokLikeAggregationWindowSeconds = 45
	settings.EventDisplayDurationMultiplier = 2.5

	// Snapshot the INTENDED values first: Update overwrites *settings from the
	// RETURNING row, so comparing against it afterwards would compare the
	// database with itself and pass no matter how the columns were bound.
	want := eventToggles(settings)

	require.NoError(t, repo.Update(ctx, settings))
	assert.Equal(t, want, eventToggles(settings), "RETURNING row disagrees with the values that were written")
	assert.Equal(t, 45, settings.TikTokLikeAggregationWindowSeconds)
	assert.InDelta(t, 2.5, settings.EventDisplayDurationMultiplier, 0.0001)

	reloaded, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, want, eventToggles(reloaded), "re-read row disagrees with the values that were written")
	assert.Equal(t, 45, reloaded.TikTokLikeAggregationWindowSeconds)
	assert.InDelta(t, 2.5, reloaded.EventDisplayDurationMultiplier, 0.0001)

	// The product case: a streamer turns coin chests off on a busy overlay and
	// nothing else may move with it.
	for _, enable := range []*bool{
		&reloaded.EnableTwitchSubs, &reloaded.EnableTwitchResubs, &reloaded.EnableTwitchGiftSubs,
		&reloaded.EnableTwitchBits, &reloaded.EnableTwitchRaids, &reloaded.EnableTwitchChannelPoints,
		&reloaded.EnableTwitchFollows, &reloaded.EnableTwitchWatchStreaks,
		&reloaded.EnableYouTubeSuperChat, &reloaded.EnableYouTubeSuperSticker, &reloaded.EnableYouTubeMembers,
		&reloaded.EnableYouTubeMemberMilestones, &reloaded.EnableYouTubeMemberGifts,
		&reloaded.EnableKickSubs, &reloaded.EnableKickGifts,
		&reloaded.EnableTikTokLikes, &reloaded.EnableTikTokGifts, &reloaded.EnableTikTokFollows,
		&reloaded.EnableTikTokShares, &reloaded.EnableTikTokTreasureChests, &reloaded.EnableTokenWarnings,
	} {
		*enable = true
	}
	require.NoError(t, repo.Update(ctx, reloaded))

	reloaded.EnableTikTokTreasureChests = false
	require.NoError(t, repo.Update(ctx, reloaded))

	final, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.NotNil(t, final)

	for column, enabled := range eventToggles(final) {
		if column == "enable_tiktok_treasure_chests" {
			assert.False(t, enabled, "disabling coin chests did not stick")
			continue
		}
		assert.True(t, enabled, "disabling coin chests also turned off %s", column)
	}
}

// eventToggles projects every boolean toggle into a DB-column-keyed map, so a
// failed comparison names the column that drifted instead of printing two
// 27-field structs.
func eventToggles(s *models.EventSettings) map[string]bool {
	return map[string]bool{
		"enable_twitch_subs":               s.EnableTwitchSubs,
		"enable_twitch_resubs":             s.EnableTwitchResubs,
		"enable_twitch_gift_subs":          s.EnableTwitchGiftSubs,
		"enable_twitch_bits":               s.EnableTwitchBits,
		"enable_twitch_raids":              s.EnableTwitchRaids,
		"enable_twitch_channel_points":     s.EnableTwitchChannelPoints,
		"enable_twitch_follows":            s.EnableTwitchFollows,
		"enable_twitch_watch_streaks":      s.EnableTwitchWatchStreaks,
		"enable_youtube_super_chat":        s.EnableYouTubeSuperChat,
		"enable_youtube_super_sticker":     s.EnableYouTubeSuperSticker,
		"enable_youtube_members":           s.EnableYouTubeMembers,
		"enable_youtube_member_milestones": s.EnableYouTubeMemberMilestones,
		"enable_youtube_member_gifts":      s.EnableYouTubeMemberGifts,
		"enable_kick_subs":                 s.EnableKickSubs,
		"enable_kick_gifts":                s.EnableKickGifts,
		"enable_tiktok_likes":              s.EnableTikTokLikes,
		"enable_tiktok_gifts":              s.EnableTikTokGifts,
		"enable_tiktok_follows":            s.EnableTikTokFollows,
		"enable_tiktok_shares":             s.EnableTikTokShares,
		"enable_tiktok_treasure_chests":    s.EnableTikTokTreasureChests,
		"enable_token_warnings":            s.EnableTokenWarnings,
	}
}

// seedOverlayWithEventSettings inserts a user + overlay and returns the overlay
// UUID. The 017 trigger creates the matching overlay_event_settings row, so the
// repository reads exactly the row production would hand it.
func seedOverlayWithEventSettings(t *testing.T, pool *pgxpool.Pool, username string) string {
	t.Helper()
	ctx := context.Background()

	var overlayID string
	err := pool.QueryRow(ctx, `
		WITH new_user AS (
			INSERT INTO users (twitch_id, username, display_name,
			                   access_token, refresh_token, token_expires_at)
			VALUES ($1, $2, $2, 'access-token', 'refresh-token', NOW() + INTERVAL '4 hours')
			RETURNING id
		)
		INSERT INTO overlays (user_id, name)
		SELECT id, 'event settings overlay' FROM new_user
		RETURNING id
	`, uuid.NewString(), username).Scan(&overlayID)
	require.NoError(t, err)

	return overlayID
}

// splitAtTreasureChestMigration splits the ordered migration set into the files
// that precede 084 and 084 plus everything after it.
func splitAtTreasureChestMigration(t *testing.T, migrations []migrationFile) (before, fromChest []migrationFile) {
	t.Helper()
	for _, m := range migrations {
		if m.name < treasureChestMigration {
			before = append(before, m)
			continue
		}
		fromChest = append(fromChest, m)
	}
	require.NotEmpty(t, before, "no migrations before %s — wrong path?", treasureChestMigration)
	require.Equal(t, treasureChestMigration, fromChest[0].name, "%s is missing from migrations/", treasureChestMigration)
	return before, fromChest
}

type migrationFile struct {
	name string
	sql  string
}

// loadUpMigrations reads migrations/[0-9]*.sql from the repo root in
// lexicographic order, skipping *_down.sql — the same selection as
// scripts/run-migrations.sh.
func loadUpMigrations(t *testing.T) []migrationFile {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "failed to read migrations dir %s", dir)

	var files []migrationFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, "_down.sql") {
			continue
		}
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "failed to read migration %s", name)
		files = append(files, migrationFile{name: name, sql: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	require.NotEmpty(t, files, "no migration files found — wrong path?")
	return files
}

func runMigrations(t *testing.T, pool *pgxpool.Pool, migrations []migrationFile) {
	t.Helper()
	ctx := context.Background()
	for _, m := range migrations {
		_, err := pool.Exec(ctx, m.sql)
		require.NoError(t, err, "migration %s failed", m.name)
	}
}

// setupEventSettingsTestDB starts a Postgres container with NO pre-created
// schema, so the real migration set owns the database like a fresh production
// cluster. The other repositories in this package hand-write a simplified
// schema, but overlay_event_settings is exactly what migration 084 changes —
// a hand-written copy would test the copy, not the migration.
func setupEventSettingsTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}
