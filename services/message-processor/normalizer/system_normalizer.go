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

	"github.com/caesar/all-chat/services/message-processor/classifier"
	"github.com/caesar/all-chat/services/message-processor/models"
)

// SystemNormalizer normalizes system events (token warnings, etc.) to unified format
type SystemNormalizer struct{}

// NewSystemNormalizer creates a new system normalizer
func NewSystemNormalizer() *SystemNormalizer {
	return &SystemNormalizer{}
}

// Normalize converts a system RawChatMessage to UnifiedChatMessage
func (n *SystemNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "system" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if raw.EventType == "" {
		return nil, fmt.Errorf("system message must have event_type")
	}

	// Normalize the event based on type
	var eventInfo *models.EventInfo
	var err error

	switch raw.EventType {
	case "token_expiration_warning":
		eventInfo, err = n.normalizeTokenWarning(raw)
	case "source_permission_error":
		eventInfo, err = n.normalizeSourcePermissionError(raw)
	default:
		return nil, fmt.Errorf("unsupported system event type: %s", raw.EventType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to normalize system event: %w", err)
	}

	// Create unified message with system user info
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "system",
		ChannelID:   "system",
		ChannelName: "All-Chat System",
		User: models.UserInfo{
			ID:          "system",
			Username:    "system",
			DisplayName: "All-Chat System",
			AvatarURL:   "",
			Badges:      []models.Badge{},
			Color:       "#EF4444", // Red for system warnings
		},
		Message: models.MessageInfo{
			Text:   "", // System events don't have chat text
			Emotes: []models.Emote{},
		},
		Timestamp: raw.Timestamp,
		Event:     eventInfo,
		Metadata:  raw.EventData,
	}

	return unified, nil
}

// normalizeSourcePermissionError normalizes a source permission error event.
// Published by the discord-listener when the bot cannot access a configured channel.
func (n *SystemNormalizer) normalizeSourcePermissionError(raw *models.RawChatMessage) (*models.EventInfo, error) {
	platform, _ := raw.EventData["platform"].(string)
	channelID, _ := raw.EventData["channel_id"].(string)
	description, _ := raw.EventData["description"].(string)

	metadata := make(map[string]interface{})
	for k, v := range raw.EventData {
		metadata[k] = v
	}
	if description == "" {
		description = fmt.Sprintf("Bot cannot access %s channel %s — check View Channel permissions.", platform, channelID)
	}
	metadata["description"] = description

	tier, duration := classifier.ClassifyEvent("system", "source_permission_error", nil)

	return &models.EventInfo{
		Type:     "source_permission_error",
		Tier:     tier,
		Duration: duration,
		IsUpdate: false,
		Metadata: metadata,
	}, nil
}

// normalizeTokenWarning normalizes a token expiration warning event
func (n *SystemNormalizer) normalizeTokenWarning(raw *models.RawChatMessage) (*models.EventInfo, error) {
	// Extract metadata from event_data
	platform, _ := raw.EventData["platform"].(string)
	username, _ := raw.EventData["username"].(string)
	failureReason, _ := raw.EventData["failure_reason"].(string)

	// Add descriptive metadata
	metadata := make(map[string]interface{})
	for k, v := range raw.EventData {
		metadata[k] = v
	}
	metadata["description"] = fmt.Sprintf("OAuth token %s for %s", failureReason, platform)
	if username != "" {
		metadata["affected_user"] = username
	}

	// Classify event - token warnings are always high priority
	tier, duration := classifier.ClassifyEvent("system", "token_expiration_warning", nil)

	// Create EventInfo
	eventInfo := &models.EventInfo{
		Type:     "token_expiration_warning",
		Tier:     tier,
		Value:    nil, // No monetary value for warnings
		Duration: duration,
		IsUpdate: false,
		Metadata: metadata,
	}

	return eventInfo, nil
}
