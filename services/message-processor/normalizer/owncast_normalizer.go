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
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// Empty the channel-ID guard for Owncast: an Owncast "channel" is an instance
// URL (ADR-0058) and carries slashes, colons and dots, so the shared
// validateChannelID ([A-Za-z0-9_-] only) must not run on it. isOwncastChannelID
// is the minimal contract: a non-empty http(s) URL.
func validateOwncastChannelID(channelID string) error {
	if channelID == "" {
		return fmt.Errorf("channel ID cannot be empty")
	}
	if len(channelID) < len("http://x.y") {
		return fmt.Errorf("channel ID is not a valid Owncast instance URL: %q", channelID)
	}
	if channelID[:7] != "http://" && (len(channelID) < 8 || channelID[:8] != "https://") {
		return fmt.Errorf("channel ID is not an http(s) URL: %q", channelID)
	}
	return nil
}

// owncastChatMessage mirrors the outbound CHAT event shape from the Owncast
// server (owncast/owncast develop, services/chat/events/userMessageEvent.go
// GetBroadcastPayload + models/user.go). Pinned by fixtures in the test file.
type owncastChatMessage struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Body      string `json:"body"`
	Visible   bool   `json:"visible"`
	User      struct {
		ID           string `json:"id"`
		DisplayName  string `json:"displayName"`
		DisplayColor int    `json:"displayColor"`
		IsBot        bool   `json:"isBot"`
	} `json:"user"`
}

// parseOwncastMessage decodes the raw_message JSON envelope.
func parseOwncastMessage(raw json.RawMessage) (*owncastChatMessage, error) {
	var msg owncastChatMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse owncast message: %w", err)
	}
	return &msg, nil
}

// OwncastNormalizer normalizes Owncast chat messages to the unified format.
type OwncastNormalizer struct{}

// NewOwncastNormalizer creates a new Owncast message normalizer.
func NewOwncastNormalizer() *OwncastNormalizer {
	return &OwncastNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage.
func (n *OwncastNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "owncast" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateOwncastChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	var msg *owncastChatMessage
	if len(raw.RawMessage) > 0 {
		if parsed, err := parseOwncastMessage(raw.RawMessage); err == nil {
			msg = parsed
		}
	}

	// The envelope is authoritative when it parses: its timestamp is the
	// server-side event time and its id the platform message id. Fall back
	// (in order) to the outer raw fields, then to a synthetic id.
	timestamp := time.Time{}
	if msg != nil && msg.Timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, msg.Timestamp); err == nil {
			timestamp = parsed
		}
	}
	if timestamp.IsZero() {
		timestamp = raw.Timestamp
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	text := raw.Text
	if text == "" && msg != nil {
		text = msg.Body
	}

	messageID := ""
	if msg != nil && msg.ID != "" {
		messageID = msg.ID
	}
	if messageID == "" {
		messageID = raw.MessageID
	}
	if messageID == "" {
		messageID = fmt.Sprintf("owncast-%d", timestamp.UnixNano())
	}

	userID := raw.UserID
	if userID == "" && msg != nil {
		userID = msg.User.ID
	}

	username := raw.Username
	if username == "" && msg != nil {
		username = msg.User.DisplayName
	}

	color := raw.Tags["color"]
	if color == "" && msg != nil && msg.User.DisplayColor > 0 {
		// Owncast gives an index into its user color palette, not a CSS
		// color; the overlay frontend maps the index. Store the raw index
		// as a string tag so nothing invents a color Owncast did not send.
		color = fmt.Sprintf("owncast:%d", msg.User.DisplayColor)
	}

	metadata := map[string]interface{}{}
	if msg != nil {
		metadata["visible"] = msg.Visible
		if msg.User.IsBot {
			metadata["is_bot"] = true
		}
	}
	if v, ok := raw.Tags["instance_url"]; ok {
		metadata["instance_url"] = v
	}

	// Owncast has no badge/emote system over chat (chat bodies are rendered
	// server-side markdown-to-HTML), so badges and emotes stay empty by
	// design; Body may contain sanitized HTML which is passed through as text.
	return &models.UnifiedChatMessage{
		ID:          messageID,
		OverlayID:   overlayID,
		Platform:    "owncast",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          userID,
			Username:    username,
			DisplayName: username,
			Color:       color,
		},
		Message: models.MessageInfo{
			Text: text,
		},
		Timestamp: timestamp,
		Metadata:  metadata,
	}, nil
}
