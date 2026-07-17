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
	"fmt"
	"regexp"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// discordEmojiRe matches Discord custom-emoji tokens in message text:
// <:name:id> (static) and <a:name:id> (animated). Names are 2-32 chars of
// [A-Za-z0-9_]; ids are snowflake digits.
var discordEmojiRe = regexp.MustCompile(`<(a?):([A-Za-z0-9_]{2,32}):(\d+)>`)

const (
	// maxDiscordAttachments bounds media items per message downstream (defence in
	// depth; the discord-listener already caps producer-side).
	maxDiscordAttachments = 4
	// maxDiscordEmotes bounds inline custom emoji per message so an emoji-spam
	// message cannot bloat the payload.
	maxDiscordEmotes = 20
)

// DiscordNormalizer normalizes Discord chat messages to unified format
type DiscordNormalizer struct{}

// NewDiscordNormalizer creates a new Discord message normalizer
func NewDiscordNormalizer() *DiscordNormalizer {
	return &DiscordNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *DiscordNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "discord" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	displayName := raw.Tags["member_nick"]
	if displayName == "" {
		displayName = raw.Username
	}

	color := raw.Tags["role_color"]
	if color == "#000000" {
		color = ""
	}

	badges := extractDiscordBadges(raw.Tags["badges"])

	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "discord",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          raw.UserID,
			Username:    raw.Username,
			DisplayName: displayName,
			Color:       color,
			Badges:      badges,
			AvatarURL:   raw.Tags["avatar_url"],
		},
		Message: models.MessageInfo{
			Text:        raw.Text,
			Emotes:      parseDiscordEmotes(raw.Text),
			Attachments: normalizeAttachments(raw.Attachments),
		},
		Timestamp: raw.Timestamp,
		Metadata:  map[string]interface{}{},
	}

	return unified, nil
}

// parseDiscordEmotes extracts inline custom-emoji tokens (<:name:id> / <a:name:id>)
// from the message text and returns them as emotes pointing at the Discord CDN.
// Positions are byte offsets with an inclusive end index, matching the convention
// used by the shared emote enricher and consumed by the frontend renderer.
// Animated emoji resolve to a .gif URL (which animates natively); static ones to .png.
func parseDiscordEmotes(text string) []models.Emote {
	matches := discordEmojiRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []models.Emote{}
	}

	emotes := make([]models.Emote, 0, len(matches))
	for _, m := range matches {
		if len(emotes) >= maxDiscordEmotes {
			break
		}
		// m indices: [fullStart, fullEnd, g1Start, g1End, g2Start, g2End, g3Start, g3End]
		fullStart, fullEnd := m[0], m[1]
		animated := m[3] > m[2] // the "a" group matched non-empty
		name := text[m[4]:m[5]]
		id := text[m[6]:m[7]]

		ext := "png"
		if animated {
			ext = "gif"
		}
		url := fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.%s?size=48&quality=lossless", id, ext)

		emotes = append(emotes, models.Emote{
			Code:      name,
			Provider:  "discord",
			URL:       url,
			Positions: [][]int{{fullStart, fullEnd - 1}},
		})
	}
	return emotes
}

// normalizeAttachments passes forwarded media through to the unified message,
// dropping entries without a usable URL and capping the count as defence in depth.
func normalizeAttachments(atts []models.Attachment) []models.Attachment {
	if len(atts) == 0 {
		return nil
	}
	out := make([]models.Attachment, 0, len(atts))
	for _, a := range atts {
		if a.URL == "" || (a.Type != "image" && a.Type != "video") {
			continue
		}
		out = append(out, a)
		if len(out) >= maxDiscordAttachments {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// extractDiscordBadges parses a comma-separated badge tag string into a Badge slice.
// Known badge values: "moderator", "admin", "vip". All badges get Version "1".
func extractDiscordBadges(badgeTag string) []models.Badge {
	badges := make([]models.Badge, 0)
	if badgeTag == "" {
		return badges
	}
	for _, name := range strings.Split(badgeTag, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			badges = append(badges, models.Badge{Name: name, Version: "1"})
		}
	}
	return badges
}
