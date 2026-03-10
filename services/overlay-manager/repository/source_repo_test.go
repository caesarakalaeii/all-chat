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
			updated_at TIMESTAMP DEFAULT NOW()
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
