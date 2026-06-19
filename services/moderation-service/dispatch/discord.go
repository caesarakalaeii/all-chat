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

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"go.uber.org/zap"
)

// discordAPI is the subset of clients.DiscordClient the dispatcher calls.
type discordAPI interface {
	DeleteMessage(ctx context.Context, channelID, messageID string) error
}

// Discord dispatches delete-only moderation to the Discord bot REST API. Unlike
// Twitch it resolves no per-user credential and performs no scope pre-check or token
// refresh: authority is the shared bot token held by the client, and the only failure
// mode that needs a human is the bot missing the MANAGE_MESSAGES permission — which is
// fixed by re-inviting the bot, not by an OAuth re-consent. Such failures are returned
// as dispatch errors so the handler responds 502 and does NOT emit a reflect-back
// (the message still exists on Discord).
type Discord struct {
	api    discordAPI
	logger *zap.Logger
}

// NewDiscord wires a Discord dispatcher.
func NewDiscord(api discordAPI, logger *zap.Logger) *Discord {
	return &Discord{api: api, logger: logger}
}

// Dispatch deletes a Discord message. Only delete is supported (the handler's
// platform-support gate enforces this; the action check here is defensive).
func (d *Discord) Dispatch(ctx context.Context, _ string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "discord" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}
	if action != models.ActionDelete {
		return models.DispatchResult{}, fmt.Errorf("dispatch: unsupported discord action %q", action)
	}

	err := d.api.DeleteMessage(ctx, req.ChannelID, req.NativeMessageID)
	switch {
	case err == nil:
		return models.DispatchResult{Outcome: models.DispatchPerformed}, nil
	case errors.Is(err, clients.ErrDiscordForbidden):
		d.logger.Warn("discord delete forbidden; the bot likely lacks the Manage Messages permission — re-invite it",
			zap.String("channel_id", req.ChannelID))
		return models.DispatchResult{}, fmt.Errorf("discord bot lacks Manage Messages permission in this channel; re-invite the bot: %w", err)
	default:
		// Unauthorized bot token or any other Discord error: a real failure. Returning
		// an error keeps the reflect-back from firing for a message that still exists.
		return models.DispatchResult{}, fmt.Errorf("discord delete failed: %w", err)
	}
}
