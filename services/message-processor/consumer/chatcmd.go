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

package consumer

import (
	"context"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

// Streamer-defined chat commands (ADR-0079): the forwarder half. The engagement
// forwarder (engagement.go) calls into these for messages on chat-command channels.

// maxChatCommandTextBytes bounds a forwarded chat-command candidate. Platforms cap
// chat at a few hundred characters; this only stops a malformed giant from riding
// the stream.
const maxChatCommandTextBytes = 2048

// chatCommandChannelsRefresh is how often the hot path reloads chatcmd:channels.
const chatCommandChannelsRefresh = 10 * time.Second

// forwardChatCommand hands a message from a chat-command channel to engagement-service
// with the chatter's roles, which only message-processor can read off the listener tags.
func (c *StreamConsumer) forwardChatCommand(ctx context.Context, raw *models.RawChatMessage) {
	if len(raw.Text) > maxChatCommandTextBytes {
		return
	}
	job := commandJob(raw)
	job.Roles = models.ChatRoles(raw)
	job.DisplayName = models.ChatDisplayName(raw)
	if err := c.xaddCommandJob(ctx, models.StreamChatCommands, job); err != nil {
		c.metrics.RecordEngagementForward("chatcmd_error")
		c.logger.Debug("forward chat command failed", zap.Error(err))
		return
	}
	c.metrics.RecordEngagementForward("chatcmd_hit")
}

// hasChatCommands reports whether the channel's streamer has an enabled chat command.
// Lock-free: the hot path reads a map the refresher swaps in whole.
func (c *StreamConsumer) hasChatCommands(platform, channelID string) bool {
	set := c.chatCmdChannels.Load()
	if set == nil {
		return false
	}
	_, ok := (*set)[models.ChatCommandChannelMember(platform, channelID)]
	return ok
}

// runChatCommandChannelsRefresher keeps the in-process copy of chatcmd:channels
// current. A new command starts working within one interval.
func (c *StreamConsumer) runChatCommandChannelsRefresher(ctx context.Context) {
	ticker := time.NewTicker(chatCommandChannelsRefresh)
	defer ticker.Stop()
	for {
		c.refreshChatCommandChannels(ctx)
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
		}
	}
}

// refreshChatCommandChannels replaces the set wholesale. On a Redis error the last
// good copy stays, so a blip neither drops nor floods forwarding.
func (c *StreamConsumer) refreshChatCommandChannels(ctx context.Context) {
	members, err := c.client.SMembers(ctx, models.ChatCommandChannelsKey).Result()
	if err != nil {
		c.logger.Debug("refresh chat command channels failed", zap.Error(err))
		return
	}
	set := make(map[string]struct{}, len(members))
	for _, m := range members {
		set[strings.ToLower(m)] = struct{}{}
	}
	c.chatCmdChannels.Store(&set)
}
