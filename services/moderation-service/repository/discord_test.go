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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two Discord reads the delegated dispatch path needs (ADR-0048). Both are inputs to an
// authorization decision that no platform re-checks afterwards, so both must fail closed on
// every path that is not an unambiguous match.

func TestDiscordIdentity(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	snowflake, found, err := repo.DiscordIdentity(ctx, ownerID)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, ownerDiscordID, snowflake)

	_, found, err = repo.DiscordIdentity(ctx, strangerID)
	require.NoError(t, err)
	assert.False(t, found, "a user who never linked Discord reports not-found, not an error")
}

// A malformed id must not reach the UUID cast and surface as a 500. It cannot match a row, so
// the honest answer is "not linked".
func TestDiscordIdentity_NonUUIDIsNotLinked(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	_, found, err := repo.DiscordIdentity(context.Background(), "not-a-uuid")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestDiscordGuildConnectedBy(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	connected, err := repo.DiscordGuildConnectedBy(ctx, ownerID, ownerGuildID)
	require.NoError(t, err)
	assert.True(t, connected, "the owner invited the bot to this guild, which is Discord's own attestation of control")

	connected, err = repo.DiscordGuildConnectedBy(ctx, ownerID, "some-other-guild")
	require.NoError(t, err)
	assert.False(t, connected, "a guild the owner never connected must not anchor anything")
}

// The anchor is what stops one streamer's delegation reaching another streamer's guild, so a
// row belonging to somebody else must not satisfy it.
func TestDiscordGuildConnectedBy_IsScopedToTheUser(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	connected, err := repo.DiscordGuildConnectedBy(context.Background(), strangerID, ownerGuildID)
	require.NoError(t, err)
	assert.False(t, connected, "another user's guild row must never anchor this user's reach")
}

func TestDiscordGuildConnectedBy_NonUUIDIsNotConnected(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	connected, err := repo.DiscordGuildConnectedBy(context.Background(), "not-a-uuid", ownerGuildID)
	require.NoError(t, err)
	assert.False(t, connected)
}
