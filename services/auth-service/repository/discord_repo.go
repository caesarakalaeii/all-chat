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
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DiscordRepository handles persistence of connected Discord guilds.
type DiscordRepository struct {
	db *pgxpool.Pool
}

// NewDiscordRepository creates a new DiscordRepository.
func NewDiscordRepository(db *pgxpool.Pool) *DiscordRepository {
	return &DiscordRepository{db: db}
}

// UpsertGuild inserts or updates a guild record for the given user.
// Uses ON CONFLICT to update guild_name and guild_icon if the (user_id, guild_id) pair exists.
func (r *DiscordRepository) UpsertGuild(ctx context.Context, guild *models.DiscordGuild) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO discord_guilds (user_id, guild_id, guild_name, guild_icon)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, guild_id)
		DO UPDATE SET guild_name = EXCLUDED.guild_name,
		              guild_icon = EXCLUDED.guild_icon
	`, guild.UserID, guild.GuildID, guild.GuildName, guild.GuildIcon)
	if err != nil {
		return fmt.Errorf("UpsertGuild: %w", err)
	}
	return nil
}

// DeleteGuild removes the guild record for the given user and guild ID.
// Returns nil if the row does not exist (idempotent — safe for best-effort disconnect flow).
func (r *DiscordRepository) DeleteGuild(ctx context.Context, userID, guildID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM discord_guilds WHERE user_id = $1 AND guild_id = $2
	`, userID, guildID)
	if err != nil {
		return fmt.Errorf("DeleteGuild: %w", err)
	}
	return nil
}

// ListGuildsByUser returns all connected Discord guilds for the given user.
func (r *DiscordRepository) ListGuildsByUser(ctx context.Context, userID string) ([]*models.DiscordGuild, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, guild_id, guild_name, guild_icon, connected_at
		FROM discord_guilds
		WHERE user_id = $1
		ORDER BY connected_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListGuildsByUser query: %w", err)
	}
	defer rows.Close()

	var guilds []*models.DiscordGuild
	for rows.Next() {
		g := &models.DiscordGuild{}
		if err := rows.Scan(&g.ID, &g.UserID, &g.GuildID, &g.GuildName, &g.GuildIcon, &g.ConnectedAt); err != nil {
			return nil, fmt.Errorf("ListGuildsByUser scan: %w", err)
		}
		guilds = append(guilds, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListGuildsByUser rows: %w", err)
	}
	return guilds, nil
}

// GetGuild returns a single guild by user_id and guild_id, or ErrNotFound if absent.
func (r *DiscordRepository) GetGuild(ctx context.Context, userID, guildID string) (*models.DiscordGuild, error) {
	g := &models.DiscordGuild{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, guild_id, guild_name, guild_icon, connected_at
		FROM discord_guilds
		WHERE user_id = $1 AND guild_id = $2
	`, userID, guildID).Scan(&g.ID, &g.UserID, &g.GuildID, &g.GuildName, &g.GuildIcon, &g.ConnectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetGuild: %w", err)
	}
	return g, nil
}

// DeleteDiscordSourcesByGuildID hard-deletes all overlay_chat_sources where platform='discord'
// and config->>'guild_id' matches the given guildID. Called during guild disconnect to clean up
// all associated chat sources for the disconnected server.
// Note: overlay_chat_sources is owned by overlay-manager but shares the same DB.
// This cross-service SQL delete is acceptable per the project's shared-DB architecture.
func (r *DiscordRepository) DeleteDiscordSourcesByGuildID(ctx context.Context, guildID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM overlay_chat_sources
		WHERE platform = 'discord' AND config->>'guild_id' = $1
	`, guildID)
	if err != nil {
		return fmt.Errorf("DeleteDiscordSourcesByGuildID: %w", err)
	}
	return nil
}
