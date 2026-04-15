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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupMaintenanceTestDB starts a PostgreSQL container and returns the repository.
func setupMaintenanceTestDB(t *testing.T) (*MaintenanceRepository, func()) {
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

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS maintenance_windows (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title       VARCHAR(200) NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			starts_at   TIMESTAMP WITH TIME ZONE NOT NULL,
			ends_at     TIMESTAMP WITH TIME ZONE NOT NULL,
			created_by  VARCHAR(100) NOT NULL,
			created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_maintenance_windows_ends_at ON maintenance_windows (ends_at);
	`)
	require.NoError(t, err)

	repo := NewMaintenanceRepository(pool)

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return repo, cleanup
}

func TestMaintenanceRepository_Create(t *testing.T) {
	repo, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("creates a maintenance window", func(t *testing.T) {
		req := models.CreateMaintenanceRequest{
			Title:       "Database upgrade",
			Description: "Upgrading PostgreSQL to 17",
			StartsAt:    now.Add(1 * time.Hour),
			EndsAt:      now.Add(3 * time.Hour),
		}

		mw, err := repo.Create(ctx, req, "admin-user-1")
		require.NoError(t, err)
		require.NotNil(t, mw)

		assert.NotEmpty(t, mw.ID)
		assert.Equal(t, "Database upgrade", mw.Title)
		assert.Equal(t, "Upgrading PostgreSQL to 17", mw.Description)
		assert.Equal(t, "admin-user-1", mw.CreatedBy)
		assert.WithinDuration(t, req.StartsAt, mw.StartsAt, time.Second)
		assert.WithinDuration(t, req.EndsAt, mw.EndsAt, time.Second)
		assert.WithinDuration(t, now, mw.CreatedAt, 5*time.Second)
	})
}

func TestMaintenanceRepository_ListAll(t *testing.T) {
	repo, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("returns empty slice when no windows exist", func(t *testing.T) {
		windows, err := repo.ListAll(ctx)
		require.NoError(t, err)
		assert.Empty(t, windows)
	})

	t.Run("returns all windows ordered by starts_at", func(t *testing.T) {
		// Create windows in reverse order to verify ordering
		_, err := repo.Create(ctx, models.CreateMaintenanceRequest{
			Title:    "Later window",
			StartsAt: now.Add(5 * time.Hour),
			EndsAt:   now.Add(6 * time.Hour),
		}, "admin")
		require.NoError(t, err)

		_, err = repo.Create(ctx, models.CreateMaintenanceRequest{
			Title:    "Earlier window",
			StartsAt: now.Add(1 * time.Hour),
			EndsAt:   now.Add(2 * time.Hour),
		}, "admin")
		require.NoError(t, err)

		windows, err := repo.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, windows, 2)
		assert.Equal(t, "Earlier window", windows[0].Title)
		assert.Equal(t, "Later window", windows[1].Title)
	})
}

func TestMaintenanceRepository_ListUpcoming(t *testing.T) {
	repo, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("excludes past windows", func(t *testing.T) {
		// Create a past window (ended 1 hour ago)
		_, err := repo.Create(ctx, models.CreateMaintenanceRequest{
			Title:    "Past window",
			StartsAt: now.Add(-3 * time.Hour),
			EndsAt:   now.Add(-1 * time.Hour),
		}, "admin")
		require.NoError(t, err)

		// Create a future window
		_, err = repo.Create(ctx, models.CreateMaintenanceRequest{
			Title:    "Future window",
			StartsAt: now.Add(1 * time.Hour),
			EndsAt:   now.Add(3 * time.Hour),
		}, "admin")
		require.NoError(t, err)

		windows, err := repo.ListUpcoming(ctx)
		require.NoError(t, err)
		require.Len(t, windows, 1)
		assert.Equal(t, "Future window", windows[0].Title)
	})
}

func TestMaintenanceRepository_Delete(t *testing.T) {
	repo, cleanup := setupMaintenanceTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("deletes an existing window", func(t *testing.T) {
		mw, err := repo.Create(ctx, models.CreateMaintenanceRequest{
			Title:    "To be deleted",
			StartsAt: now.Add(1 * time.Hour),
			EndsAt:   now.Add(2 * time.Hour),
		}, "admin")
		require.NoError(t, err)

		err = repo.Delete(ctx, mw.ID)
		assert.NoError(t, err)

		// Verify it's gone
		windows, err := repo.ListAll(ctx)
		require.NoError(t, err)
		for _, w := range windows {
			assert.NotEqual(t, mw.ID, w.ID)
		}
	})

	t.Run("returns ErrMaintenanceNotFound for nonexistent ID", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, ErrMaintenanceNotFound)
	})
}
