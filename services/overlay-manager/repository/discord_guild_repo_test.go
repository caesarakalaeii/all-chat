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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupDiscordGuildTestDatabase starts PostgreSQL and creates discord_guilds as migration 035
// defines it, so the ownership query is exercised against the real column names and the
// UNIQUE(user_id, guild_id) shape it relies on.
func setupDiscordGuildTestDatabase(t *testing.T) (*DiscordGuildRepository, func()) {
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

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS discord_guilds (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			guild_id VARCHAR(32) NOT NULL,
			guild_name VARCHAR(200),
			guild_icon TEXT,
			connected_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (user_id, guild_id)
		)`)
	require.NoError(t, err)

	return NewDiscordGuildRepository(pool), func() {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}
}

// TestUserOwnsGuild verifies the authorization anchor for every Discord source: the row must
// belong to the asking user, and another user's connection to the same guild must not count.
func TestUserOwnsGuild(t *testing.T) {
	repo, cleanup := setupDiscordGuildTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	owner := uuid.New().String()
	stranger := uuid.New().String()

	_, err := repo.pool.Exec(ctx,
		`INSERT INTO discord_guilds (user_id, guild_id, guild_name) VALUES ($1, $2, $3)`,
		owner, "guild-123", "Owner's Server")
	require.NoError(t, err)

	t.Run("connected guild is owned", func(t *testing.T) {
		owns, err := repo.UserOwnsGuild(ctx, owner, "guild-123")
		require.NoError(t, err)
		assert.True(t, owns)
	})

	t.Run("another user's guild is not owned", func(t *testing.T) {
		owns, err := repo.UserOwnsGuild(ctx, stranger, "guild-123")
		require.NoError(t, err)
		assert.False(t, owns,
			"a guild connected by someone else must never authorize this user")
	})

	t.Run("unconnected guild is not owned", func(t *testing.T) {
		owns, err := repo.UserOwnsGuild(ctx, owner, "guild-999")
		require.NoError(t, err)
		assert.False(t, owns)
	})

	t.Run("empty guild id is not owned", func(t *testing.T) {
		owns, err := repo.UserOwnsGuild(ctx, owner, "")
		require.NoError(t, err)
		assert.False(t, owns,
			"an empty guild id must not match — the client returns a sentinel error instead")
	})
}
