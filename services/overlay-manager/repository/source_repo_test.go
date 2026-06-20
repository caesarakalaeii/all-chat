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
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupSourceTestDatabase starts a PostgreSQL container and returns the source repository.
// It creates the overlays, overlay_chat_sources, and share_requests tables.
func setupSourceTestDatabase(t *testing.T) (*SourceRepository, func()) {
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

	repo, err := NewSourceRepository(connStr)
	require.NoError(t, err)

	_, err = repo.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS overlays (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT TRUE,
			is_public_for_viewers BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS share_requests (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			requester_id UUID NOT NULL,
			target_id UUID NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS overlay_chat_sources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
			platform VARCHAR(50) NOT NULL,
			channel_id VARCHAR(100) NOT NULL,
			channel_name VARCHAR(100) NOT NULL,
			channel_handle VARCHAR(100),
			auth_required BOOLEAN DEFAULT FALSE,
			config JSONB DEFAULT '{}'::jsonb,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(overlay_id, platform, channel_id)
		);

		-- Minimal users table (owned by auth-service; shared DB in prod). Required so the
		-- chat_via_eventsub subquery in ListByOverlayID resolves.
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(50) NOT NULL,
			auth_provider VARCHAR(20) NOT NULL DEFAULT 'twitch',
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			token_expires_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		-- Linked Twitch credentials (ADR-0016, migration 056): lets non-Twitch-login
		-- accounts satisfy the chat_via_eventsub predicate for their channel.
		CREATE TABLE IF NOT EXISTS twitch_oauth_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			twitch_user_id VARCHAR(50) NOT NULL,
			twitch_login VARCHAR(100) NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			encryption_version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, twitch_login)
		);
	`)
	require.NoError(t, err)

	cleanup := func() {
		repo.pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return repo, cleanup
}

// createTestOverlay inserts a minimal overlay row and returns its ID.
func createTestOverlay(t *testing.T, repo *SourceRepository) string {
	t.Helper()
	ctx := context.Background()
	overlayID := uuid.New().String()
	_, err := repo.pool.Exec(ctx,
		`INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, $3)`,
		overlayID, uuid.New().String(), "Test Overlay",
	)
	require.NoError(t, err)
	return overlayID
}

// TestCreateOrUpdateAuto_Idempotent verifies that re-adding the same (overlay, platform,
// channel_id) via the auto path does not error or duplicate — it returns the existing row,
// refreshes display fields, and re-activates. This is what makes the OAuth re-consent flow
// (e.g. enabling EventSub chat on an already-configured channel) succeed instead of failing.
func TestCreateOrUpdateAuto_Idempotent(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := createTestOverlay(t, repo)

	src := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   "caesarlp",
		ChannelName: "CaesarLP",
		Config:      map[string]interface{}{},
		IsActive:    true,
	}
	require.NoError(t, repo.CreateOrUpdateAuto(ctx, src))
	firstID := src.ID
	require.NotEmpty(t, firstID)

	// Re-consent: same overlay/platform/channel, different display name.
	dup := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   "caesarlp",
		ChannelName: "CaesarLP Updated",
		Config:      map[string]interface{}{},
		IsActive:    true,
	}
	require.NoError(t, repo.CreateOrUpdateAuto(ctx, dup), "re-consent must not error on the UNIQUE constraint")
	require.Equal(t, firstID, dup.ID, "must return the existing source id, not create a new row")

	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1, "must remain exactly one source row")
	require.Equal(t, "CaesarLP Updated", sources[0].ChannelName, "display fields refreshed on re-consent")
	require.True(t, sources[0].IsActive)
}

// TestListByOverlayID_NonSharedSource verifies that a regular (non-shared_overlay) source
// has ShareStatus == nil and the JSON omitempty tag keeps share_status absent.
func TestListByOverlayID_NonSharedSource(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := createTestOverlay(t, repo)

	src := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   "chan1",
		ChannelName: "streamer1",
		IsActive:    true,
	}
	require.NoError(t, repo.Create(ctx, src))

	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	assert.Equal(t, "twitch", sources[0].Platform)
	assert.Nil(t, sources[0].ShareStatus, "ShareStatus must be nil for non-shared_overlay sources")
}

// TestListByOverlayID_SharedOverlaySource_Active verifies that a shared_overlay source
// gets its share_status populated from the share_requests table.
func TestListByOverlayID_SharedOverlaySource_Active(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := createTestOverlay(t, repo)

	// Insert a share_request row with status 'active'
	shareID := uuid.New().String()
	_, err := repo.pool.Exec(ctx,
		`INSERT INTO share_requests (id, requester_id, target_id, status) VALUES ($1, $2, $3, $4)`,
		shareID, uuid.New().String(), uuid.New().String(), "active",
	)
	require.NoError(t, err)

	// Insert a shared_overlay source where channel_id = shareID (Phase 16 convention)
	src := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "shared_overlay",
		ChannelID:   shareID,
		ChannelName: "Partner Overlay",
		IsActive:    true,
	}
	require.NoError(t, repo.Create(ctx, src))

	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	assert.Equal(t, "shared_overlay", sources[0].Platform)
	require.NotNil(t, sources[0].ShareStatus, "ShareStatus must be non-nil for shared_overlay sources")
	assert.Equal(t, "active", *sources[0].ShareStatus)
}

// TestListByOverlayID_SharedOverlaySource_Revoked verifies that a revoked shared_overlay
// source returns share_status = 'revoked'.
func TestListByOverlayID_SharedOverlaySource_Revoked(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := createTestOverlay(t, repo)

	shareID := uuid.New().String()
	_, err := repo.pool.Exec(ctx,
		`INSERT INTO share_requests (id, requester_id, target_id, status) VALUES ($1, $2, $3, $4)`,
		shareID, uuid.New().String(), uuid.New().String(), "revoked",
	)
	require.NoError(t, err)

	src := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "shared_overlay",
		ChannelID:   shareID,
		ChannelName: "Revoked Overlay",
		IsActive:    false,
	}
	require.NoError(t, repo.Create(ctx, src))

	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	require.NotNil(t, sources[0].ShareStatus)
	assert.Equal(t, "revoked", *sources[0].ShareStatus)
}

// TestListByOverlayID_MixedSources verifies that in a list with both regular and
// shared_overlay sources, only the shared ones have ShareStatus set.
func TestListByOverlayID_MixedSources(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := createTestOverlay(t, repo)

	// Regular twitch source
	twitchSrc := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   "twitch-chan",
		ChannelName: "TwitchStreamer",
		IsActive:    true,
	}
	require.NoError(t, repo.Create(ctx, twitchSrc))

	// shared_overlay source with a known share_request
	shareID := uuid.New().String()
	_, err := repo.pool.Exec(ctx,
		`INSERT INTO share_requests (id, requester_id, target_id, status) VALUES ($1, $2, $3, $4)`,
		shareID, uuid.New().String(), uuid.New().String(), "expired",
	)
	require.NoError(t, err)

	sharedSrc := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "shared_overlay",
		ChannelID:   shareID,
		ChannelName: "Shared Partner",
		IsActive:    false,
	}
	require.NoError(t, repo.Create(ctx, sharedSrc))

	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 2)

	for _, s := range sources {
		if s.Platform == "twitch" {
			assert.Nil(t, s.ShareStatus, "twitch source must not have ShareStatus")
		} else if s.Platform == "shared_overlay" {
			require.NotNil(t, s.ShareStatus, "shared_overlay source must have ShareStatus")
			assert.Equal(t, "expired", *s.ShareStatus)
		}
	}
}

// TestListByOverlayID_ChatViaEventSub_LinkedCredentials covers the ADR-0016
// path: a YouTube/Kick-login account has no users row matching its Twitch
// channel, but a twitch_oauth_tokens row (written by the Twitch add-source
// link flow) must flip chat_via_eventsub to true — and the frontend badge
// with it.
func TestListByOverlayID_ChatViaEventSub_LinkedCredentials(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()
	ctx := context.Background()

	overlayID := createTestOverlay(t, repo)
	source := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   "blvtumi",
		ChannelName: "BLVTumi",
		Config:      map[string]interface{}{},
	}
	require.NoError(t, repo.Create(ctx, source))

	// No users row, no linked credentials: stays on IRC.
	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.False(t, sources[0].ChatViaEventSub, "no credentials anywhere — must not claim EventSub")

	// Linked credentials with chat scope and valid expiry (login case differs).
	_, err = repo.pool.Exec(ctx, `
		INSERT INTO twitch_oauth_tokens
			(user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES (gen_random_uuid(), '555', 'BLVTumi', 'enc-access', 'enc-refresh',
		        NOW() + INTERVAL '2 hours', ARRAY['user:read:chat','user:bot','channel:bot'])
	`)
	require.NoError(t, err)

	sources, err = repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.True(t, sources[0].ChatViaEventSub, "linked twitch_oauth_tokens row must satisfy the predicate")

	// Expired linked credentials must NOT satisfy the predicate.
	_, err = repo.pool.Exec(ctx, `UPDATE twitch_oauth_tokens SET token_expires_at = NOW() - INTERVAL '1 hour'`)
	require.NoError(t, err)

	sources, err = repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.False(t, sources[0].ChatViaEventSub, "expired linked credentials must not claim EventSub")
}

// TestListByOverlayIDForUser_IsOwnChannel covers the per-requesting-user ownership flag
// that drives the IRC→EventSub re-consent nudge. Ownership is independent of whether the
// channel already reads via EventSub or whether tokens are valid — it only means "the
// requester can re-consent for this channel". Two ownership paths must both work: a
// Twitch-login users row, and a linked twitch_oauth_tokens row (ADR-0016, the case the
// old frontend isOwnChannel heuristic missed).
func TestListByOverlayIDForUser_IsOwnChannel(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()
	ctx := context.Background()

	overlayID := createTestOverlay(t, repo)
	source := &models.ChatSource{
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   "caesarlp",
		ChannelName: "CaesarLP",
		Config:      map[string]interface{}{},
	}
	require.NoError(t, repo.Create(ctx, source))

	// No identity passed: never owned (public/admin callers).
	sources, err := repo.ListByOverlayIDForUser(ctx, overlayID, "")
	require.NoError(t, err)
	require.False(t, sources[0].IsOwnChannel, "empty requesting user must never own a channel")

	// A different user does not own this channel.
	stranger := uuid.New().String()
	_, err = repo.pool.Exec(ctx,
		`INSERT INTO users (id, username, auth_provider) VALUES ($1, 'someoneelse', 'twitch')`, stranger)
	require.NoError(t, err)
	sources, err = repo.ListByOverlayIDForUser(ctx, overlayID, stranger)
	require.NoError(t, err)
	require.False(t, sources[0].IsOwnChannel, "unrelated user must not own the channel")

	// Twitch-login path: a users row whose username matches the channel id.
	twitchOwner := uuid.New().String()
	_, err = repo.pool.Exec(ctx,
		`INSERT INTO users (id, username, auth_provider) VALUES ($1, 'CaesarLP', 'twitch')`, twitchOwner)
	require.NoError(t, err)
	sources, err = repo.ListByOverlayIDForUser(ctx, overlayID, twitchOwner)
	require.NoError(t, err)
	require.True(t, sources[0].IsOwnChannel, "twitch-login owner (username == channel_id) must own the channel")

	// Linked-credentials path: a non-Twitch-login user who linked this Twitch channel.
	linkedOwner := uuid.New().String()
	_, err = repo.pool.Exec(ctx,
		`INSERT INTO users (id, username, auth_provider) VALUES ($1, 'kicklogin', 'kick')`, linkedOwner)
	require.NoError(t, err)
	_, err = repo.pool.Exec(ctx, `
		INSERT INTO twitch_oauth_tokens
			(user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1, '777', 'CaesarLP', 'enc-access', 'enc-refresh', NOW() - INTERVAL '1 hour', ARRAY['user:read:chat'])
	`, linkedOwner)
	require.NoError(t, err)
	sources, err = repo.ListByOverlayIDForUser(ctx, overlayID, linkedOwner)
	require.NoError(t, err)
	require.True(t, sources[0].IsOwnChannel,
		"linked twitch_oauth_tokens owner must own the channel even with an expired token (re-consent is the point)")
}

// TestListByOverlayID_DefaultsIsOwnChannelFalse ensures the back-compat method that omits a
// requesting user never reports ownership.
func TestListByOverlayID_DefaultsIsOwnChannelFalse(t *testing.T) {
	repo, cleanup := setupSourceTestDatabase(t)
	defer cleanup()
	ctx := context.Background()

	overlayID := createTestOverlay(t, repo)
	require.NoError(t, repo.Create(ctx, &models.ChatSource{
		OverlayID: overlayID, Platform: "twitch", ChannelID: "caesarlp",
		ChannelName: "CaesarLP", Config: map[string]interface{}{},
	}))

	sources, err := repo.ListByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.False(t, sources[0].IsOwnChannel)
}
