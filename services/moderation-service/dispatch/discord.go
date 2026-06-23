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

package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"go.uber.org/zap"
)

// discordAPI is the subset of clients.DiscordClient the dispatcher calls. Member ops
// (timeout/ban/unban) are guild-scoped; the guild id is resolved from the channel id by
// guilds, not threaded through the platform-agnostic DispatchRequest.
type discordAPI interface {
	DeleteMessage(ctx context.Context, channelID, messageID string) error
	TimeoutMember(ctx context.Context, guildID, userID string, until time.Time) error
	BanMember(ctx context.Context, guildID, userID string) error
	UnbanMember(ctx context.Context, guildID, userID string) error
}

// discordGuildResolver maps a channel id to its guild id (clients.DiscordGuildResolver).
type discordGuildResolver interface {
	GuildID(ctx context.Context, channelID string) (string, error)
}

// Discord dispatches moderation (delete + member ban/timeout/unban) to the Discord bot
// REST API. Unlike Twitch it resolves no per-user credential and performs no OAuth scope
// pre-check or token refresh: authority is the shared bot token, and the only failure
// mode that needs a human is the bot missing a moderation permission (MANAGE_MESSAGES /
// MODERATE_MEMBERS / BAN_MEMBERS) — fixed by RE-INVITING the bot with the elevated
// permissions, not by an OAuth re-consent. Such failures are returned as dispatch errors
// so the handler responds 502 and does NOT emit a reflect-back (the action did not land).
// The capability endpoint reports the bot's real permissions, so the dashboard normally
// shows the re-invite prompt before a 403 can happen here.
type Discord struct {
	api    discordAPI
	guilds discordGuildResolver
	logger *zap.Logger
}

// NewDiscord wires a Discord dispatcher over the bot REST client and a channel→guild
// resolver.
func NewDiscord(api discordAPI, guilds discordGuildResolver, logger *zap.Logger) *Discord {
	return &Discord{api: api, guilds: guilds, logger: logger}
}

// Dispatch performs a Discord moderation action. delete is channel-scoped; timeout/ban/
// unban are guild-scoped, so the guild id is resolved from the channel id first.
func (d *Discord) Dispatch(ctx context.Context, _ string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "discord" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}

	if action == models.ActionDelete {
		return d.result(action, req.ChannelID, d.api.DeleteMessage(ctx, req.ChannelID, req.NativeMessageID))
	}

	// Member ops need the guild id. Resolution failure that is a permission/visibility
	// problem maps to the re-invite path; anything else is a transient dispatch error.
	guildID, err := d.guilds.GuildID(ctx, req.ChannelID)
	if err != nil {
		return d.result(action, req.ChannelID, fmt.Errorf("resolve guild for channel: %w", err))
	}

	switch action {
	case models.ActionTimeout:
		until := time.Now().Add(time.Duration(req.DurationSeconds) * time.Second)
		return d.result(action, req.ChannelID, d.api.TimeoutMember(ctx, guildID, req.TargetUserID, until))
	case models.ActionBan:
		return d.result(action, req.ChannelID, d.api.BanMember(ctx, guildID, req.TargetUserID))
	case models.ActionUnban:
		return d.result(action, req.ChannelID, d.api.UnbanMember(ctx, guildID, req.TargetUserID))
	default:
		return models.DispatchResult{}, fmt.Errorf("dispatch: unsupported discord action %q", action)
	}
}

// result classifies a Discord API outcome. A 403 (bot lacks the moderation permission)
// is returned as an error with a re-invite hint — never a false reflect-back. nil is
// success (DispatchPerformed).
func (d *Discord) result(action models.Action, channelID string, err error) (models.DispatchResult, error) {
	switch {
	case err == nil:
		return models.DispatchResult{Outcome: models.DispatchPerformed}, nil
	case errors.Is(err, clients.ErrDiscordForbidden):
		d.logger.Warn("discord moderation forbidden; the bot lacks the required permission — re-invite it with moderation permissions",
			zap.String("action", string(action)), zap.String("channel_id", channelID))
		return models.DispatchResult{}, fmt.Errorf("discord bot lacks the permission for %s; re-invite the bot with moderation permissions: %w", action, err)
	default:
		// Unauthorized bot token or any other error: a real failure. Returning an error
		// keeps the reflect-back from firing for an action that did not land.
		return models.DispatchResult{}, fmt.Errorf("discord %s failed: %w", action, err)
	}
}
