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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	ownerID    = "11111111-1111-1111-1111-111111111111"
	strangerID = "22222222-2222-2222-2222-222222222222"
	overlayID  = "aaaaaaaa-1111-1111-1111-111111111111"
)

func TestVerifyOverlayOwnership(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	owns, err := repo.VerifyOverlayOwnership(ctx, overlayID, ownerID)
	require.NoError(t, err)
	assert.True(t, owns, "the owner must pass the ownership check")

	owns, err = repo.VerifyOverlayOwnership(ctx, overlayID, strangerID)
	require.NoError(t, err)
	assert.False(t, owns, "a non-owner must fail the ownership check")
}

func TestListModeratableSources_ExcludesSharedOverlay(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	sources, err := repo.ListModeratableSources(ctx, overlayID)
	require.NoError(t, err)

	platforms := make(map[string]string) // platform -> channel_id
	for _, s := range sources {
		platforms[s.Platform] = s.ChannelID
	}
	assert.Equal(t, "somestreamer", platforms["twitch"])
	assert.Equal(t, "tiktokuser", platforms["tiktok"], "tiktok is listed; the capability handler marks it unsupported")
	_, hasShared := platforms["shared_overlay"]
	assert.False(t, hasShared, "shared_overlay sources must never be listed as moderatable")
}

func TestIsModeratableSource(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	ok, err := repo.IsModeratableSource(ctx, overlayID, "twitch", "somestreamer")
	require.NoError(t, err)
	assert.True(t, ok, "a real twitch source on the overlay is moderatable")

	ok, err = repo.IsModeratableSource(ctx, overlayID, "twitch", "not-on-this-overlay")
	require.NoError(t, err)
	assert.False(t, ok, "a channel not configured on the overlay must be rejected")

	ok, err = repo.IsModeratableSource(ctx, overlayID, "shared_overlay", "some-share-id")
	require.NoError(t, err)
	assert.False(t, ok, "shared_overlay sources must be rejected even when present")
}

func TestIsUserPremium(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	premium, err := repo.IsUserPremium(ctx, ownerID)
	require.NoError(t, err)
	assert.True(t, premium, "the seeded owner is premium")

	premium, err = repo.IsUserPremium(ctx, strangerID)
	require.NoError(t, err)
	assert.False(t, premium, "a non-premium user reports false")

	_, err = repo.IsUserPremium(ctx, "33333333-3333-3333-3333-333333333333")
	assert.Error(t, err, "an unknown user is an error (fails closed at the gate)")
}

// setupTestRepo spins up a throwaway Postgres with a minimal schema and seeds an
// overlay owned by ownerID carrying twitch, tiktok, and shared_overlay sources.
func setupTestRepo(t *testing.T) (*Repository, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start postgres container")

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	require.NoError(t, err)

	const schema = `
		CREATE TABLE users (
			id UUID PRIMARY KEY,
			is_premium BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE overlays (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE overlay_chat_sources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL,
			platform VARCHAR(50) NOT NULL,
			channel_id VARCHAR(100) NOT NULL,
			channel_name VARCHAR(100) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true
		);`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err)

	// owner is premium (in the rollout cohort); stranger is a non-premium user.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, is_premium) VALUES ($1, true), ($2, false)`, ownerID, strangerID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO overlays (id, user_id) VALUES ($1, $2)`, overlayID, ownerID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name) VALUES
		($1, 'twitch', 'somestreamer', 'SomeStreamer'),
		($1, 'tiktok', 'tiktokuser', 'TikTokUser'),
		($1, 'shared_overlay', 'some-share-id', 'A Friend')`, overlayID)
	require.NoError(t, err)

	return New(pool), func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}
