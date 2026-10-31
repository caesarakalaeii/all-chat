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
	"strings"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicateTrigger is returned when a user already has a command or keyword with
// the same trigger (case-insensitive).
var ErrDuplicateTrigger = errors.New("a chat command with this trigger already exists")

const chatCommandCols = `id, user_id, kind, trigger, response, create_clip, image_url, min_role,
	cooldown_seconds, enabled, use_count, created_at, updated_at`

// scanChatCommand scans chatCommandCols, after any leading columns into lead.
func scanChatCommand(row pgx.Row, lead ...any) (models.ChatCommand, error) {
	var c models.ChatCommand
	err := row.Scan(append(lead, &c.ID, &c.UserID, &c.Kind, &c.Trigger, &c.Response, &c.CreateClip,
		&c.ImageURL, &c.MinRole, &c.CooldownSeconds, &c.Enabled, &c.UseCount, &c.CreatedAt, &c.UpdatedAt)...)
	return c, err
}

func collectChatCommands(rows pgx.Rows) ([]models.ChatCommand, error) {
	defer rows.Close()
	out := []models.ChatCommand{}
	for rows.Next() {
		c, err := scanChatCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chat command: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListChatCommands returns every command of a user, oldest first (the order keyword
// triggers are tried in).
func (r *Repository) ListChatCommands(ctx context.Context, userID uuid.UUID) ([]models.ChatCommand, error) {
	rows, err := r.db.Query(ctx, `SELECT `+chatCommandCols+` FROM chat_commands
		WHERE user_id = $1 ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list chat commands: %w", err)
	}
	return collectChatCommands(rows)
}

// CountChatCommands returns how many commands a user has (the per-user cap).
func (r *Repository) CountChatCommands(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM chat_commands WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

// CreateChatCommand inserts c for c.UserID.
func (r *Repository) CreateChatCommand(ctx context.Context, c models.ChatCommand) (models.ChatCommand, error) {
	out, err := scanChatCommand(r.db.QueryRow(ctx, `INSERT INTO chat_commands
		(user_id, kind, trigger, response, create_clip, image_url, min_role, cooldown_seconds, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING `+chatCommandCols,
		c.UserID, c.Kind, c.Trigger, c.Response, c.CreateClip, c.ImageURL, c.MinRole, c.CooldownSeconds, c.Enabled))
	return out, mapChatCommandErr(err)
}

// UpdateChatCommand replaces the editable fields of a command the user owns.
// ErrNotFound when the id does not exist or belongs to someone else.
func (r *Repository) UpdateChatCommand(ctx context.Context, c models.ChatCommand) (models.ChatCommand, error) {
	out, err := scanChatCommand(r.db.QueryRow(ctx, `UPDATE chat_commands SET
		kind = $3, trigger = $4, response = $5, create_clip = $6, image_url = $7, min_role = $8,
		cooldown_seconds = $9, enabled = $10, updated_at = now()
		WHERE id = $1 AND user_id = $2 RETURNING `+chatCommandCols,
		c.ID, c.UserID, c.Kind, c.Trigger, c.Response, c.CreateClip, c.ImageURL, c.MinRole, c.CooldownSeconds, c.Enabled))
	return out, mapChatCommandErr(err)
}

// DeleteChatCommand removes a command the user owns. ErrNotFound otherwise.
func (r *Repository) DeleteChatCommand(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM chat_commands WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete chat command: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// IncrementChatCommandUse bumps use_count and returns the new value ({count}).
func (r *Repository) IncrementChatCommandUse(ctx context.Context, id uuid.UUID) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `UPDATE chat_commands SET use_count = use_count + 1
		WHERE id = $1 RETURNING use_count`, id).Scan(&n)
	return n, err
}

func mapChatCommandErr(err error) error {
	var pgErr *pgconn.PgError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return ErrDuplicateTrigger
	}
	return fmt.Errorf("save chat command: %w", err)
}

// AlertsOverlays returns the ids of the user's alerts overlays, where a chat
// command's image shows (ADR-0080).
func (r *Repository) AlertsOverlays(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM overlays WHERE user_id = $1 AND overlay_type = 'alerts'
		ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("alerts overlays: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// chatSourceMatch matches a chat source row s against a raw message's channel ($2).
// Kick listeners stamp the slug (channel_handle, else channel_name) rather than
// channel_id, so Kick matches either.
const chatSourceMatch = `s.platform = $1 AND s.is_active AND (LOWER(s.channel_id) = LOWER($2)
	OR (s.platform = 'kick' AND LOWER(COALESCE(s.channel_handle, s.channel_name)) = LOWER($2)))`

// channelOwned decides whether the matched channel is the overlay owner's own channel
// on that platform, with the same predicates moderation-service and the streamer send
// path use: Twitch the users row of a Twitch login (the send/clip token lives there),
// YouTube the owner's youtube_oauth_tokens, Kick a Kick login or kick_oauth_tokens.
const channelOwned = `CASE s.platform
	WHEN 'twitch' THEN EXISTS (SELECT 1 FROM users u WHERE u.id = o.user_id
		AND u.auth_provider = 'twitch' AND LOWER(u.username) = LOWER(s.channel_id))
	WHEN 'youtube' THEN EXISTS (SELECT 1 FROM youtube_oauth_tokens y WHERE y.user_id = o.user_id
		AND y.channel_id = s.channel_id)
	WHEN 'kick' THEN EXISTS (SELECT 1 FROM users u WHERE u.id = o.user_id
		AND u.auth_provider = 'kick' AND LOWER(u.username) = LOWER($2))
		OR EXISTS (SELECT 1 FROM kick_oauth_tokens k WHERE k.user_id = o.user_id
		AND LOWER(k.channel_id) = LOWER($2))
	ELSE FALSE END`

// ChannelCommands is the chat commands (and message-counting timers, ADR-0081) one
// user has on a channel's chat.
type ChannelCommands struct {
	UserID uuid.UUID
	// Owned: the channel is the user's own, so a reply (sent to their own channel)
	// lands in the chat the command came from. Commands on someone else's channel in
	// the user's overlay still fire, but only their clip runs.
	Owned bool
	// TimerMessages: the user runs an enabled timer with min_messages on the channel
	// they own, so every chat line here counts toward that timer (ADR-0081). The
	// channel arrives in chatcmd:channels because of the timer even with no command.
	TimerMessages bool
	Commands      []models.ChatCommand
}

// ChatCommandsForChannel returns, per user whose overlays carry this channel, that
// user's enabled commands (oldest first) and whether an enabled message-counting
// timer of theirs watches their own channel here (ADR-0081). One query: it runs for
// every forwarded message of a chat-command channel.
func (r *Repository) ChatCommandsForChannel(ctx context.Context, platform, channelID string) ([]ChannelCommands, error) {
	rows, err := r.db.Query(ctx, `WITH owners AS (
			SELECT o.user_id,
				bool_or(`+channelOwned+`) AS owned,
				bool_or(`+channelOwned+` AND EXISTS (SELECT 1 FROM chat_timers t
					WHERE t.user_id = o.user_id AND t.enabled AND t.min_messages IS NOT NULL)) AS counts
			FROM overlay_chat_sources s JOIN overlays o ON o.id = s.overlay_id
			WHERE `+chatSourceMatch+`
			GROUP BY o.user_id)
		SELECT owned, counts, `+chatCommandCols+`
		FROM chat_commands JOIN owners USING (user_id)
		WHERE enabled
		ORDER BY user_id, created_at, id`, platform, channelID)
	if err != nil {
		return nil, fmt.Errorf("chat commands for channel: %w", err)
	}
	defer rows.Close()
	var out []ChannelCommands
	for rows.Next() {
		var owned, counts bool
		c, err := scanChatCommand(rows, &owned, &counts)
		if err != nil {
			return nil, fmt.Errorf("scan channel chat command: %w", err)
		}
		if n := len(out); n == 0 || out[n-1].UserID != c.UserID {
			out = append(out, ChannelCommands{UserID: c.UserID, Owned: owned, TimerMessages: counts})
		}
		out[len(out)-1].Commands = append(out[len(out)-1].Commands, c)
	}
	return out, rows.Err()
}

// ChatCommandChannels returns the chatcmd:channels members: every active chat source,
// as platform:channel, on an overlay whose owner has an enabled command, or whose
// owner runs an enabled timer that counts chat messages (ADR-0081: the counter lives
// where the messages are forwarded, so those channels must arrive too). Kick sources
// also contribute their slug, which is what the Kick listener stamps on messages.
func (r *Repository) ChatCommandChannels(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT s.platform, s.channel_id,
			COALESCE(s.channel_handle, s.channel_name, '')
		FROM overlay_chat_sources s JOIN overlays o ON o.id = s.overlay_id
		WHERE s.is_active AND s.platform <> $1
		  AND (EXISTS (SELECT 1 FROM chat_commands c WHERE c.user_id = o.user_id AND c.enabled)
		   OR EXISTS (SELECT 1 FROM chat_timers t WHERE t.user_id = o.user_id
			AND t.enabled AND t.min_messages IS NOT NULL))`,
		sharedOverlayPlatform)
	if err != nil {
		return nil, fmt.Errorf("chat command channels: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var platform, channelID, slug string
		if err := rows.Scan(&platform, &channelID, &slug); err != nil {
			return nil, fmt.Errorf("scan chat command channel: %w", err)
		}
		out = append(out, platform+":"+channelID)
		if platform == "kick" && slug != "" && !strings.EqualFold(slug, channelID) {
			out = append(out, platform+":"+slug)
		}
	}
	return out, rows.Err()
}

// ChatCommandOwnerStatus is what the editor shows about the owner's account: whether
// replies on YouTube are allowed (premium, because they spend YouTube API quota) and
// which Twitch grants the reply and clip paths need.
type ChatCommandOwnerStatus struct {
	IsPremium       bool `json:"is_premium"`
	TwitchConnected bool `json:"twitch_connected"`
	TwitchCanSend   bool `json:"twitch_can_send"`
	TwitchCanClip   bool `json:"twitch_can_clip"`
	// ConsentOverlayID is an overlay carrying the streamer's own Twitch channel. The
	// Twitch grant flow (auth-service /twitch/moderation/:overlay_id) re-links the
	// source on that overlay, so the editor needs one to offer the clip grant.
	ConsentOverlayID *uuid.UUID `json:"consent_overlay_id"`
}

// OwnerStatus reads ChatCommandOwnerStatus from the users row. Only a Twitch login
// counts as Twitch-connected: auth-service sends and clips with the users-row token.
func (r *Repository) OwnerStatus(ctx context.Context, userID uuid.UUID) (ChatCommandOwnerStatus, error) {
	var s ChatCommandOwnerStatus
	err := r.db.QueryRow(ctx, `SELECT u.is_premium,
			u.auth_provider = 'twitch' AND u.twitch_id IS NOT NULL,
			u.auth_provider = 'twitch' AND 'user:write:chat' = ANY(u.granted_scopes),
			u.auth_provider = 'twitch' AND 'clips:edit' = ANY(u.granted_scopes),
			(SELECT o.id FROM overlays o JOIN overlay_chat_sources s ON s.overlay_id = o.id
			 WHERE o.user_id = u.id AND u.auth_provider = 'twitch' AND s.platform = 'twitch'
			   AND LOWER(s.channel_id) = LOWER(u.username)
			 ORDER BY o.created_at LIMIT 1)
		FROM users u WHERE u.id = $1`, userID).
		Scan(&s.IsPremium, &s.TwitchConnected, &s.TwitchCanSend, &s.TwitchCanClip, &s.ConsentOverlayID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}
