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
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DiscordGuildRepository reads the discord_guilds connections written by auth-service's
// bot-invite callback (migration 035).
type DiscordGuildRepository struct {
	pool *pgxpool.Pool
}

// NewDiscordGuildRepository creates a repository over an existing pool.
func NewDiscordGuildRepository(pool *pgxpool.Pool) *DiscordGuildRepository {
	return &DiscordGuildRepository{pool: pool}
}

// UserOwnsGuild reports whether the user has connected this guild to All-Chat.
//
// A row exists only for the user who completed the bot-invite flow for that guild, and
// Discord offers that flow only to members holding Manage Server — so the row is the closest
// available proof that the user administers the guild. It is the authorization anchor for
// every Discord source, because Discord itself authorizes the shared bot rather than the
// caller and will not refuse a channel the caller has no claim to (ADR-0048).
func (r *DiscordGuildRepository) UserOwnsGuild(ctx context.Context, userID, guildID string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM discord_guilds
			WHERE user_id = $1 AND guild_id = $2
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, userID, guildID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check discord guild ownership: %w", err)
	}
	return exists, nil
}
