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
	GuildID     string    `json:"guild_id"`   // Discord Snowflake ID — always string
	GuildName   string    `json:"guild_name"`
	GuildIcon   *string   `json:"guild_icon"` // nullable CDN hash
	ConnectedAt time.Time `json:"connected_at"`
}
