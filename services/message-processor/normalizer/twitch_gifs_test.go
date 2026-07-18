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

package normalizer

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Twitch chat GIFs (ADR-0037): the "gifs" tag carries "start-end|gif_id|url", the bracketed
// alt caption occupies that span in the text, and Twitch renders the GIF in its place.
func TestTwitchNormalizer_Normalize_Gifs(t *testing.T) {
	normalizer := NewTwitchNormalizer()
	const gifURL = "https://media4.giphy.com/media/joSNxeswxuc74Juo8X/giphy.gif?cid=abc&ct=g"

	t.Run("standalone gif becomes an attachment and clears the caption text", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g1",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "[Y A Y Yes GIF by Djemilah Birnie]", // 34 bytes, offsets 0-33
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{"gifs": "0-33|joSNxeswxuc74Juo8X|" + gifURL},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		require.Len(t, msg.Message.Attachments, 1)
		att := msg.Message.Attachments[0]
		assert.Equal(t, "image", att.Type)
		assert.Equal(t, "image/gif", att.ContentType)
		assert.Equal(t, gifURL, att.URL)
		assert.Equal(t, "Y A Y Yes GIF by Djemilah Birnie", att.Filename) // brackets trimmed
		assert.Equal(t, "", msg.Message.Text)                            // caption stripped
	})

	t.Run("gif keeps surrounding text and strips only its span", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g2",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "look [cat]", // "[cat]" at offsets 5-9
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{"gifs": "5-9|abc|" + gifURL},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		require.Len(t, msg.Message.Attachments, 1)
		assert.Equal(t, "cat", msg.Message.Attachments[0].Filename)
		assert.Equal(t, "look ", msg.Message.Text)
	})

	t.Run("first-party emote after a gif is re-anchored to the stripped text", func(t *testing.T) {
		// "[cat] Kappa": "[cat]" at 0-4, " " at 5, "Kappa" at 6-10. After stripping the GIF,
		// the text is " Kappa" and Kappa moves from 6-10 to 1-5.
		raw := &models.RawChatMessage{
			MessageID: "g3",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "[cat] Kappa",
			Timestamp: time.Now().UTC(),
			Tags: map[string]string{
				"gifs":   "0-4|abc|" + gifURL,
				"emotes": "25:6-10",
			},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		assert.Equal(t, " Kappa", msg.Message.Text)
		require.Len(t, msg.Message.Attachments, 1)
		require.Len(t, msg.Message.Emotes, 1)
		emote := msg.Message.Emotes[0]
		assert.Equal(t, "Kappa", emote.Code)
		assert.Equal(t, [][]int{{1, 5}}, emote.Positions)
	})

	t.Run("multiple gifs yield multiple attachments in order", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g4",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "[a] [bb]", // "[a]" 0-2, " " 3, "[bb]" 4-7
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{"gifs": "0-2|id1|https://x/1.gif,4-7|id2|https://x/2.gif"},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		require.Len(t, msg.Message.Attachments, 2)
		assert.Equal(t, "https://x/1.gif", msg.Message.Attachments[0].URL)
		assert.Equal(t, "https://x/2.gif", msg.Message.Attachments[1].URL)
		assert.Equal(t, " ", msg.Message.Text) // both captions stripped, separator space remains
	})

	t.Run("no gifs tag leaves text and attachments untouched", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g5",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "just a normal message",
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		assert.Equal(t, "just a normal message", msg.Message.Text)
		assert.Empty(t, msg.Message.Attachments)
	})

	t.Run("multibyte text before the gif keeps byte offsets correct", func(t *testing.T) {
		// "日本 " is 7 bytes (3+3+1); "[cat]" then occupies bytes 7-11.
		raw := &models.RawChatMessage{
			MessageID: "g7",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "日本 [cat]",
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{"gifs": "7-11|abc|" + gifURL},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		require.Len(t, msg.Message.Attachments, 1)
		assert.Equal(t, "cat", msg.Message.Attachments[0].Filename)
		assert.Equal(t, "日本 ", msg.Message.Text)
	})

	t.Run("caption whose title contains brackets strips only the outer pair", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g8",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "[[Meta] Reaction GIF]", // 21 bytes, offsets 0-20
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{"gifs": "0-20|abc|" + gifURL},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		require.Len(t, msg.Message.Attachments, 1)
		assert.Equal(t, "[Meta] Reaction GIF", msg.Message.Attachments[0].Filename)
		assert.Equal(t, "", msg.Message.Text)
	})

	t.Run("non-https gif url is dropped", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g9",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "[cat]",
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{"gifs": "0-4|abc|http://insecure.example/x.gif"},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		assert.Empty(t, msg.Message.Attachments)
		assert.Equal(t, "[cat]", msg.Message.Text) // nothing stripped
	})

	t.Run("malformed span still renders the gif without stripping", func(t *testing.T) {
		raw := &models.RawChatMessage{
			MessageID: "g6",
			Platform:  "twitch",
			ChannelID: "streamer",
			UserID:    "1",
			Username:  "viewer",
			Text:      "[cat]",
			Timestamp: time.Now().UTC(),
			// End offset past the text length → span not strippable.
			Tags: map[string]string{"gifs": "0-99|abc|" + gifURL},
		}

		msg, err := normalizer.Normalize(raw, "overlay-1")
		require.NoError(t, err)

		require.Len(t, msg.Message.Attachments, 1)
		assert.Equal(t, gifURL, msg.Message.Attachments[0].URL)
		assert.Equal(t, "[cat]", msg.Message.Text) // unchanged
	})
}
