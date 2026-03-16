//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	repo := repository.NewViewerIdentityRepository(pool)
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
	repo := repository.NewViewerIdentityRepository(pool)
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
	repo := repository.NewViewerIdentityRepository(pool)
	ctx := context.Background()

	// Use a random UUID that has no cosmetics row
	viewerID := uuid.New()

	color, err := repo.GetViewerCosmetics(ctx, viewerID)
	require.NoError(t, err)
	assert.Nil(t, color, "expected nil color for viewer with no cosmetics row")
}

func TestUpsertViewerCosmetics(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.NewViewerIdentityRepository(pool)
	ctx := context.Background()

	platform := "twitch"
	platformUserID := "user_cosmetics_test"
	defer cleanupViewer(t, pool, platform, platformUserID)

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	require.NoError(t, err)

	expectedColor := "#ff6600"
	err = repo.UpsertViewerCosmetics(ctx, viewerID, &expectedColor)
	require.NoError(t, err)

	color, err := repo.GetViewerCosmetics(ctx, viewerID)
	require.NoError(t, err)
	require.NotNil(t, color)
	assert.Equal(t, expectedColor, *color)
}
