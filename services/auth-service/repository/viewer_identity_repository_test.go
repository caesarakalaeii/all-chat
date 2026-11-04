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

package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/premium"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestDB returns a pgxpool connected to the local test database.
// Requires a running PostgreSQL instance with allchat database.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := "postgres://allchat:allchat_dev_password@localhost:5432/allchat"
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("requires DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("requires DB: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// cleanupViewer removes test data for the given platform/user pair.
func cleanupViewer(t *testing.T, pool *pgxpool.Pool, platform, platformUserID string) {
	t.Helper()
	ctx := context.Background()
	// Delete via viewer_platform_identities → viewers cascade will handle the rest
	pool.Exec(ctx, `
		DELETE FROM viewers WHERE id IN (
			SELECT viewer_id FROM viewer_platform_identities
			WHERE platform = $1 AND platform_user_id = $2
		)
	`, platform, platformUserID)
}

func TestGetOrCreateViewerByPlatform_NewViewer(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()

	platform := "twitch"
	platformUserID := "user_test_123"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, viewerID, "expected a non-nil viewer UUID")
}

func TestGetOrCreateViewerByPlatform_Existing(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()

	platform := "twitch"
	platformUserID := "user_test_idempotent"
	defer cleanupViewer(t, pool, platform, platformUserID)

	first, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)

	second, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)

	assert.Equal(t, first, second, "repeated calls must return the same viewer_id")
}

func TestGetViewerCosmetics_NotFound(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()

	// Use a random UUID that has no cosmetics row
	viewerID := uuid.New()

	color, err := repo.GetViewerCosmetics(ctx, viewerID)
	require.NoError(t, err)
	assert.Nil(t, color, "expected nil color for viewer with no cosmetics row")
}

func TestUpsertViewerCosmetics(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()

	platform := "twitch"
	platformUserID := "user_cosmetics_test"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)

	expectedColor := "#ff6600"
	_, err = repo.UpsertViewerCosmetics(ctx, viewerID, repository.CosmeticsUpdate{SetName: true, NameColor: &expectedColor})
	require.NoError(t, err)

	color, err := repo.GetViewerCosmetics(ctx, viewerID)
	require.NoError(t, err)
	require.NotNil(t, color)
	assert.Equal(t, expectedColor, *color)
}

// TestUpsertViewerCosmetics_ZeroUUIDViolatesAvatarFK is the DB-layer regression
// guard for the prod bug (reported 2026-07-17) where the cosmetics handler passed
// a pointer to uuid.Nil as a "clear" sentinel. pgx encodes a non-nil pointer as
// the literal '00000000-...' value, which has no cosmetic_frames / cosmetic_flairs
// row and violates the avatar FK constraints (SQLSTATE 23503) — 500'ing every
// non-premium save. Passing nil (→ SQL NULL) must succeed. This asserts both.
func TestUpsertViewerCosmetics_ZeroUUIDViolatesAvatarFK(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()

	platform := "twitch"
	platformUserID := "user_cosmetics_fk_test"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)

	color := "#00ff00"

	// nil avatar pointers → SQL NULL → satisfies the FK → succeeds (the fix).
	_, err = repo.UpsertViewerCosmetics(ctx, viewerID, repository.CosmeticsUpdate{
		SetName: true, NameColor: &color,
		SetFrame: true, AvatarFrameID: nil,
		SetFlair: true, AvatarFlairID: nil,
	})
	require.NoError(t, err, "nil avatar pointers should write SQL NULL and satisfy the FK")

	// A pointer to the zero UUID → literal '00000000-...' → FK violation (the bug).
	zero := uuid.Nil
	_, err = repo.UpsertViewerCosmetics(ctx, viewerID, repository.CosmeticsUpdate{
		SetFrame: true, AvatarFrameID: &zero,
		SetFlair: true, AvatarFlairID: &zero,
	})
	require.Error(t, err, "zero-UUID avatar ids must violate the avatar FK constraint")
}

// TestUpsertViewerCosmetics_PartialUpdatePreservesUntouchedColumns verifies the
// per-column PATCH semantics against a real database: a name-only update must not
// disturb a saved avatar frame (the ON CONFLICT CASE gating), and the RETURNING
// clause must report the full merged row.
func TestUpsertViewerCosmetics_PartialUpdatePreservesUntouchedColumns(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()

	platform := "twitch"
	platformUserID := "user_cosmetics_partial_test"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)

	// Seed a catalog frame so a valid avatar_frame_id can be set.
	var frameID uuid.UUID
	err = pool.QueryRow(ctx, `INSERT INTO cosmetic_frames (name, image_url) VALUES ('itest', 'itest') RETURNING id`).Scan(&frameID)
	require.NoError(t, err)
	defer pool.Exec(ctx, `DELETE FROM cosmetic_frames WHERE id = $1`, frameID)

	// Set the avatar frame.
	_, err = repo.UpsertViewerCosmetics(ctx, viewerID, repository.CosmeticsUpdate{SetFrame: true, AvatarFrameID: &frameID})
	require.NoError(t, err)

	// A name-only update must preserve the frame and return the merged row.
	color := "#123456"
	res, err := repo.UpsertViewerCosmetics(ctx, viewerID, repository.CosmeticsUpdate{SetName: true, NameColor: &color})
	require.NoError(t, err)
	require.NotNil(t, res.NameColor)
	assert.Equal(t, color, *res.NameColor)
	require.NotNil(t, res.AvatarFrameID, "name-only update must preserve the saved avatar frame")
	assert.Equal(t, frameID, *res.AvatarFrameID)
}

// seedEngagementIdentity creates a viewers row plus its platform identity without a
// viewer session, the way engagement-service does for a chatter who earns points
// before ever signing in.
func seedEngagementIdentity(t *testing.T, pool *pgxpool.Pool, platform, platformUserID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var viewerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO viewers DEFAULT VALUES RETURNING id`).Scan(&viewerID))
	_, err := pool.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, $2, $3)`,
		viewerID, platform, platformUserID)
	require.NoError(t, err)
	return viewerID
}

// seedSession inserts a viewer_sessions row with the given viewer_id (nil = NULL)
// and removes it when the test ends.
func seedSession(t *testing.T, pool *pgxpool.Pool, platform, platformUserID string, viewerID *uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name, access_token, token_expires_at, viewer_id)
		VALUES ($1, $2, 'itest', 'itest', 'x', NOW(), $3)`, platform, platformUserID, viewerID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM viewer_sessions WHERE platform = $1 AND platform_user_id = $2`, platform, platformUserID)
	})
}

func sessionViewerID(t *testing.T, pool *pgxpool.Pool, platform, platformUserID string) *uuid.UUID {
	t.Helper()
	var viewerID *uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT viewer_id FROM viewer_sessions WHERE platform = $1 AND platform_user_id = $2`,
		platform, platformUserID).Scan(&viewerID))
	return viewerID
}

// A chatter who earned points first already has an identity when they sign in.
// Resolving it must link their new session to that identity, or the admin premium
// grant (which resolves the viewer through the session) has nothing to act on.
func TestGetOrCreateViewerByPlatform_ExistingIdentityLinksSession(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	platform, platformUserID := "youtube", "UC_itest_engagement_first"
	defer cleanupViewer(t, pool, platform, platformUserID)

	existing := seedEngagementIdentity(t, pool, platform, platformUserID)
	seedSession(t, pool, platform, platformUserID, nil)

	viewerID, err := repo.GetOrCreateViewerByPlatform(context.Background(), platform, platformUserID)
	require.NoError(t, err)
	assert.Equal(t, existing, viewerID)
	got := sessionViewerID(t, pool, platform, platformUserID)
	require.NotNil(t, got, "session must be linked to the existing identity")
	assert.Equal(t, existing, *got)
}

// Moving a platform to another viewer (connect flow) must move its session too, or
// premium granted through the session lands on the old viewer while the badge
// enricher reads the new one.
func TestLinkPlatformToViewer_MoveRelinksSession(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	platform, platformUserID := "kick", "itest_move_session"
	defer cleanupViewer(t, pool, platform, platformUserID)

	oldViewer := seedEngagementIdentity(t, pool, platform, platformUserID)
	seedSession(t, pool, platform, platformUserID, &oldViewer)
	var newViewer uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(), `INSERT INTO viewers DEFAULT VALUES RETURNING id`).Scan(&newViewer))
	defer pool.Exec(context.Background(), `DELETE FROM viewers WHERE id = $1`, newViewer)

	require.NoError(t, repo.LinkPlatformToViewer(context.Background(), newViewer, platform, platformUserID))
	got := sessionViewerID(t, pool, platform, platformUserID)
	require.NotNil(t, got)
	assert.Equal(t, newViewer, *got)
}

// Premium lives on the durable viewer, so an admin can grant it to an identity
// that has no session at all (ADR-0034).
func TestSetViewerPremium_SessionlessIdentity(t *testing.T) {
	pool := newTestDB(t)
	recomputer := premium.NewRecomputer(pool, zap.NewNop())
	repo := repository.NewViewerRepository(pool, nil, recomputer)
	ctx := context.Background()
	platform, platformUserID := "youtube", "UC_itest_sessionless_grant"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID := seedEngagementIdentity(t, pool, platform, platformUserID)
	require.NoError(t, repo.SetViewerPremium(ctx, viewerID, true, nil))

	var isPremium bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT is_premium FROM viewers WHERE id = $1`, viewerID).Scan(&isPremium))
	assert.True(t, isPremium)

	require.NoError(t, repo.SetViewerPremium(ctx, viewerID, false, nil))
	require.NoError(t, pool.QueryRow(ctx, `SELECT is_premium FROM viewers WHERE id = $1`, viewerID).Scan(&isPremium))
	assert.False(t, isPremium)
}

func TestSetViewerPremium_UnknownViewer(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerRepository(pool, nil, premium.NewRecomputer(pool, zap.NewNop()))
	err := repo.SetViewerPremium(context.Background(), uuid.New(), true, nil)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

// New viewers start unaccepted (migration 126) and accepting persists.
func TestViewerTos_AcceptRoundTrip(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))
	ctx := context.Background()
	platform, platformUserID := "twitch", "itest_viewer_tos"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)
	accepted, err := repo.GetViewerTosAccepted(ctx, viewerID)
	require.NoError(t, err)
	assert.False(t, accepted, "a new viewer must be prompted")

	require.NoError(t, repo.AcceptViewerTos(ctx, viewerID))
	accepted, err = repo.GetViewerTosAccepted(ctx, viewerID)
	require.NoError(t, err)
	assert.True(t, accepted)

	unknown := uuid.New()
	_, err = repo.GetViewerTosAccepted(ctx, unknown)
	assert.ErrorIs(t, err, repository.ErrNotFound)
	assert.ErrorIs(t, repo.AcceptViewerTos(ctx, unknown), repository.ErrNotFound)
}
