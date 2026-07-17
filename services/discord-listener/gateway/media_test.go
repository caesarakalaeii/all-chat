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

package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyMediaType(t *testing.T) {
	cases := map[string]string{
		"image/png":       "image",
		"image/jpeg":      "image",
		"image/gif":       "image",
		"image/webp":      "image",
		"video/mp4":       "video",
		"video/webm":      "video",
		"application/pdf": "",
		"audio/mpeg":      "",
		"":                "",
	}
	for in, want := range cases {
		assert.Equalf(t, want, classifyMediaType(in), "classifyMediaType(%q)", in)
	}
}

func TestBuildRawAttachments_UploadedImage(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{{
			Filename:    "cat.png",
			ContentType: "image/png",
			URL:         "https://cdn.discordapp.com/attachments/1/2/cat.png",
			ProxyURL:    "https://media.discordapp.net/attachments/1/2/cat.png",
			Width:       800,
			Height:      600,
		}},
	}

	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.Equal(t, "image", got[0].Type)
	// proxy_url is preferred over url
	assert.Equal(t, "https://media.discordapp.net/attachments/1/2/cat.png", got[0].URL)
	assert.Equal(t, "image/png", got[0].ContentType)
	assert.Equal(t, 800, got[0].Width)
	assert.Equal(t, 600, got[0].Height)
	assert.False(t, got[0].Spoiler)
	assert.Equal(t, "cat.png", got[0].Filename)
}

func TestBuildRawAttachments_GifIsImage(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{{
			Filename:    "dance.gif",
			ContentType: "image/gif",
			ProxyURL:    "https://media.discordapp.net/dance.gif",
		}},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.Equal(t, "image", got[0].Type, "GIFs are images that animate natively")
}

func TestBuildRawAttachments_Video(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{{
			Filename:    "clip.mp4",
			ContentType: "video/mp4",
			ProxyURL:    "https://media.discordapp.net/clip.mp4",
		}},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.Equal(t, "video", got[0].Type)
}

func TestBuildRawAttachments_Spoiler(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{{
			Filename:    "SPOILER_ending.png",
			ContentType: "image/png",
			ProxyURL:    "https://media.discordapp.net/ending.png",
		}},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.True(t, got[0].Spoiler)
	assert.Equal(t, "ending.png", got[0].Filename, "spoiler prefix stripped from filename")
}

func TestBuildRawAttachments_FallsBackToURL(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{{
			Filename:    "cat.png",
			ContentType: "image/png",
			URL:         "https://cdn.discordapp.com/cat.png",
			// no ProxyURL
		}},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.Equal(t, "https://cdn.discordapp.com/cat.png", got[0].URL)
}

func TestBuildRawAttachments_DropsNonMediaAndURLLess(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{
			{Filename: "notes.pdf", ContentType: "application/pdf", ProxyURL: "https://x/notes.pdf"},
			{Filename: "empty.png", ContentType: "image/png"}, // no url at all
		},
	}
	got := buildRawAttachments(msg)
	assert.Empty(t, got)
}

func TestBuildRawAttachments_GifvEmbed(t *testing.T) {
	msg := MessageCreateData{
		Embeds: []DiscordEmbed{{
			Type: "gifv",
			URL:  "https://tenor.com/view/foo",
			Video: &DiscordEmbedMedia{
				URL:      "https://media.tenor.com/foo.mp4",
				ProxyURL: "https://media.discordapp.net/external/abc/foo.mp4",
				Width:    498,
				Height:   280,
			},
			Thumbnail: &DiscordEmbedMedia{
				ProxyURL: "https://media.discordapp.net/external/abc/foo.png",
			},
		}},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.Equal(t, "video", got[0].Type)
	assert.Equal(t, "https://media.discordapp.net/external/abc/foo.mp4", got[0].URL, "embed proxy_url preferred")
	assert.Equal(t, "https://media.discordapp.net/external/abc/foo.png", got[0].ThumbURL)
	assert.Equal(t, 498, got[0].Width)
}

func TestBuildRawAttachments_ImageEmbed(t *testing.T) {
	msg := MessageCreateData{
		Embeds: []DiscordEmbed{{
			Type:  "image",
			Image: &DiscordEmbedMedia{ProxyURL: "https://media.discordapp.net/ext/pic.png", Width: 100, Height: 100},
		}},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, 1)
	assert.Equal(t, "image", got[0].Type)
	assert.Equal(t, "https://media.discordapp.net/ext/pic.png", got[0].URL)
}

func TestBuildRawAttachments_IgnoresNonMediaEmbeds(t *testing.T) {
	msg := MessageCreateData{
		Embeds: []DiscordEmbed{
			{Type: "article", URL: "https://news.example/story"},
			{Type: "link", URL: "https://example.com"},
			{Type: "rich"},
			{Type: "gifv"}, // gifv with no video object
			{Type: "image"}, // image with no image object
		},
	}
	got := buildRawAttachments(msg)
	assert.Empty(t, got)
}

func TestBuildRawAttachments_CapAndOrder(t *testing.T) {
	msg := MessageCreateData{
		Attachments: []DiscordAttachment{
			{Filename: "a.png", ContentType: "image/png", ProxyURL: "https://x/a.png"},
			{Filename: "b.png", ContentType: "image/png", ProxyURL: "https://x/b.png"},
			{Filename: "c.png", ContentType: "image/png", ProxyURL: "https://x/c.png"},
			{Filename: "d.png", ContentType: "image/png", ProxyURL: "https://x/d.png"},
			{Filename: "e.png", ContentType: "image/png", ProxyURL: "https://x/e.png"},
		},
		Embeds: []DiscordEmbed{
			{Type: "image", Image: &DiscordEmbedMedia{ProxyURL: "https://x/embed.png"}},
		},
	}
	got := buildRawAttachments(msg)
	require.Len(t, got, maxAttachmentsPerMessage)
	// uploads come before embeds and the cap trims the tail
	assert.Equal(t, "https://x/a.png", got[0].URL)
	assert.Equal(t, "https://x/d.png", got[3].URL)
}

func TestBuildRawAttachments_Empty(t *testing.T) {
	assert.Empty(t, buildRawAttachments(MessageCreateData{}))
}
