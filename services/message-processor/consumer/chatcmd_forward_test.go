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
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChatCmdTestConsumer(t *testing.T, mr *miniredis.Miniredis) *StreamConsumer {
	c := newTestConsumer(t, mr)
	c.engagementCh = make(chan *models.RawChatMessage, 4)
	return c
}

// A channel with chat commands forwards ordinary chat (keyword triggers match any
// text) with the chatter's roles; a channel without them forwards nothing.
func TestChatCommandForwarding(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	ctx := context.Background()
	c := newChatCmdTestConsumer(t, mr)

	_, err = mr.SAdd(models.ChatCommandChannelsKey, "twitch:caesarlp")
	require.NoError(t, err)
	c.refreshChatCommandChannels(ctx)
	assert.True(t, c.hasChatCommands("Twitch", "CaesarLP"))
	assert.False(t, c.hasChatCommands("twitch", "other"))

	other := &models.RawChatMessage{Platform: "twitch", ChannelID: "other", Text: "clip that"}
	c.forwardEngagementCommand(other)
	assert.Len(t, c.engagementCh, 0, "ordinary chat on a channel without commands is not forwarded")

	raw := &models.RawChatMessage{
		MessageID: "m1", Platform: "twitch", ChannelID: "caesarlp", UserID: "42", Username: "bob",
		Text: "clip that", Tags: map[string]string{"badges": "vip/1", "display-name": "Bob"},
	}
	c.forwardEngagementCommand(raw)
	require.Len(t, c.engagementCh, 1)
	c.forwardEngagementNow(ctx, <-c.engagementCh)

	entries, err := c.client.XRange(ctx, models.StreamChatCommands, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	var job models.CommandJob
	require.NoError(t, json.Unmarshal([]byte(entries[0].Values[models.FieldEngagementData].(string)), &job))
	assert.Equal(t, "clip that", job.Text)
	assert.Equal(t, []string{"vip"}, job.Roles)
	assert.Equal(t, "Bob", job.DisplayName)

	// Not command-shaped, so the vote stream stays untouched.
	n, err := c.client.XLen(ctx, models.StreamEngagementCommands).Result()
	require.NoError(t, err)
	assert.Zero(t, n)
}

// The refresher replaces the set wholesale, so a channel whose last command was
// deleted stops forwarding.
func TestChatCommandChannelsRefreshDropsRemoved(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	ctx := context.Background()
	c := newChatCmdTestConsumer(t, mr)

	_, _ = mr.SAdd(models.ChatCommandChannelsKey, "kick:somebody")
	c.refreshChatCommandChannels(ctx)
	require.True(t, c.hasChatCommands("kick", "somebody"))

	_, _ = mr.SRem(models.ChatCommandChannelsKey, "kick:somebody")
	c.refreshChatCommandChannels(ctx)
	assert.False(t, c.hasChatCommands("kick", "somebody"))
}
