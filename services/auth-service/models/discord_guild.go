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

package models

import "time"

// DiscordGuild represents a connected Discord server in the discord_guilds table.
// GuildID is a string — Discord Snowflake IDs exceed JavaScript's safe integer range (2^53),
// so they MUST be stored and transmitted as strings, never as int64 or uint64.
type DiscordGuild struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	GuildID     string    `json:"guild_id"` // Discord Snowflake ID — always string
	GuildName   string    `json:"guild_name"`
	GuildIcon   *string   `json:"guild_icon"` // nullable CDN hash
	ConnectedAt time.Time `json:"connected_at"`
}

// DiscordIdentity links an All-Chat account to a Discord USER account (migration 083, ADR-0048).
//
// Distinct from DiscordGuild, which records which SERVERS a streamer connected: the bot-invite
// flow returns a guild and no user identity, so this is the only place All-Chat learns who
// someone is on Discord. It is required because Discord has no per-user moderation API — the
// shared bot performs every write, so verifying that the acting human may moderate means reading
// *their* guild permissions, which needs their snowflake.
//
// No OAuth token is held: the identify grant is used once, at link time, and discarded.
type DiscordIdentity struct {
	UserID string `json:"user_id"`
	// DiscordUserID is a Snowflake — always a string, for the reason on DiscordGuild.GuildID.
	DiscordUserID string `json:"discord_user_id"`
	// DiscordUsername is display only. Discord usernames are mutable, so never identify a user
	// by this; refreshed on every re-link.
	DiscordUsername string    `json:"discord_username"`
	LinkedAt        time.Time `json:"linked_at"`
}
