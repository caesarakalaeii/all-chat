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

package gateway_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/caesar/all-chat/services/discord-listener/publisher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mediaCapturePublisher records the last raw message map so a test can assert on
// what HandleMessageCreate actually published.
type mediaCapturePublisher struct {
	last  interface{}
	calls int
}

func (p *mediaCapturePublisher) Publish(_ context.Context, msg interface{}) error {
	p.calls++
	p.last = msg
	return nil
}

// roundTripToRawMessage mirrors the production publisherAdapter: it marshals the
// gateway's map[string]interface{} and unmarshals it into publisher.RawMessage.
// This exercises the cross-struct JSON contract end to end.
func roundTripToRawMessage(t *testing.T, published interface{}) publisher.RawMessage {
	t.Helper()
	data, err := json.Marshal(published)
	require.NoError(t, err)
	var raw publisher.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	return raw
}

func TestHandleMessageCreate_ForwardsAttachments(t *testing.T) {
	pub := &mediaCapturePublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token", "wss://gateway.discord.gg", store, nil, reg, pub, nil,
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-1",
		ChannelID: "channel-1",
		Content:   "look at this",
		Author:    gateway.DiscordUser{ID: "user-1", Username: "someone"},
		Attachments: []gateway.DiscordAttachment{{
			Filename:    "SPOILER_cat.gif",
			ContentType: "image/gif",
			ProxyURL:    "https://media.discordapp.net/cat.gif",
			Width:       200,
			Height:      150,
		}},
		Embeds: []gateway.DiscordEmbed{{
			Type:      "gifv",
			Video:     &gateway.DiscordEmbedMedia{ProxyURL: "https://media.discordapp.net/ext/foo.mp4", Width: 480, Height: 270},
			Thumbnail: &gateway.DiscordEmbedMedia{ProxyURL: "https://media.discordapp.net/ext/foo.png"},
		}},
	}

	require.NoError(t, client.HandleMessageCreate(context.Background(), msg))
	require.Equal(t, 1, pub.calls)

	raw := roundTripToRawMessage(t, pub.last)
	require.Len(t, raw.Attachments, 2, "both the upload and the gifv embed must survive the wire contract")

	// Uploaded image (GIF), spoiler-flagged.
	assert.Equal(t, "image", raw.Attachments[0].Type)
	assert.Equal(t, "https://media.discordapp.net/cat.gif", raw.Attachments[0].URL)
	assert.Equal(t, "image/gif", raw.Attachments[0].ContentType)
	assert.True(t, raw.Attachments[0].Spoiler)
	assert.Equal(t, "cat.gif", raw.Attachments[0].Filename)

	// Tenor/Giphy-style gifv embed -> looping video with a poster.
	assert.Equal(t, "video", raw.Attachments[1].Type)
	assert.Equal(t, "https://media.discordapp.net/ext/foo.mp4", raw.Attachments[1].URL)
	assert.Equal(t, "https://media.discordapp.net/ext/foo.png", raw.Attachments[1].ThumbURL)
}

// TestHandleMessageCreate_ImageOnlyPublishes guards the empty-content fix: a
// media-only message (empty content) must still publish rather than being
// misread as a disabled MESSAGE_CONTENT intent.
func TestHandleMessageCreate_ImageOnlyPublishes(t *testing.T) {
	pub := &mediaCapturePublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token", "wss://gateway.discord.gg", store, nil, reg, pub, nil,
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-2",
		ChannelID: "channel-1",
		Content:   "", // image-only message
		Author:    gateway.DiscordUser{ID: "user-1", Username: "someone"},
		Attachments: []gateway.DiscordAttachment{{
			Filename:    "pic.png",
			ContentType: "image/png",
			ProxyURL:    "https://media.discordapp.net/pic.png",
		}},
	}

	require.NoError(t, client.HandleMessageCreate(context.Background(), msg))
	require.Equal(t, 1, pub.calls, "media-only message must publish")

	raw := roundTripToRawMessage(t, pub.last)
	require.Len(t, raw.Attachments, 1)
	assert.Equal(t, "https://media.discordapp.net/pic.png", raw.Attachments[0].URL)
}

// TestHandleMessageCreate_NoAttachmentsOmitsField confirms plain text messages
// carry no attachments field (wire format unchanged for the common case).
func TestHandleMessageCreate_NoAttachmentsOmitsField(t *testing.T) {
	pub := &mediaCapturePublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token", "wss://gateway.discord.gg", store, nil, reg, pub, nil,
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-3",
		ChannelID: "channel-1",
		Content:   "just text",
		Author:    gateway.DiscordUser{ID: "user-1", Username: "someone"},
	}

	require.NoError(t, client.HandleMessageCreate(context.Background(), msg))
	require.Equal(t, 1, pub.calls)

	raw := roundTripToRawMessage(t, pub.last)
	assert.Empty(t, raw.Attachments)
}
