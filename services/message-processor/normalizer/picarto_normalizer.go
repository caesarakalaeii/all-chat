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

	"github.com/caesar/all-chat/services/message-processor/models"
)

// picartoEmoteRe matches Picarto emote shortcodes in message text, e.g.
// :ptv-sleepy: or :ptv-hearts:. Emotes are OUT OF SCOPE for Picarto (plan
// excludes them): shortcodes are stripped from the text and the message
// renders as plain text.
var picartoEmoteRe = regexp.MustCompile(`:[A-Za-z0-9_\-]{2,40}:`)

// PicartoNormalizer normalizes Picarto chat messages to unified format.
type PicartoNormalizer struct{}

// NewPicartoNormalizer creates a new Picarto message normalizer.
func NewPicartoNormalizer() *PicartoNormalizer {
	return &PicartoNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage.
func (n *PicartoNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "picarto" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	displayName := raw.Tags["display_name"]
	if displayName == "" {
		displayName = raw.Username
	}

	// Emote shortcodes are stripped; the text stays plain.
	text := picartoEmoteRe.ReplaceAllString(raw.Text, "")

	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "picarto",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          raw.UserID,
			Username:    raw.Username,
			DisplayName: displayName,
			AvatarURL:   raw.Tags["avatar_url"],
			Color:       picartoColor(raw.Tags["color"]),
		},
		Message: models.MessageInfo{
			Text:   text,
			Emotes: []models.Emote{},
		},
		Timestamp: raw.Timestamp,
		Metadata:  map[string]interface{}{},
	}

	return unified, nil
}

// picartoColor normalizes the hex colour the WS frame carries. The server
// sends 6 hex digits without '#'; the unified format expects '#rrggbb'.
func picartoColor(raw string) string {
	if raw == "" {
		return ""
	}
	if raw[0] == '#' {
		return raw
	}
	return "#" + raw
}
