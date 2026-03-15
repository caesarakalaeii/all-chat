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
