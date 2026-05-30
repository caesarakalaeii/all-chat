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

import "github.com/caesar/all-chat/services/message-processor/models"

// Normalizer defines the interface for platform-specific normalizers
type Normalizer interface {
	Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error)
}

// NormalizeDeletion converts RawChatMessage deletion event to UnifiedChatMessage
// This is a shared function used by all platforms since deletion format is unified
func NormalizeDeletion(raw *models.RawChatMessage) *models.UnifiedChatMessage {
	deletionType, ok := raw.EventData["deletion_type"].(string)
	if !ok {
		deletionType = "unknown"
	}

	// Build event metadata
	eventMetadata := map[string]interface{}{
		"deletion_type": deletionType,
	}

	// Add type-specific fields
	switch deletionType {
	case "single":
		// Internal UUID added by consumer (from registry lookup)
		if uuid, ok := raw.EventData["target_uuid"].(string); ok {
			eventMetadata["target_uuid"] = uuid
		}
		if msgID, ok := raw.EventData["target_msg_id"].(string); ok {
			eventMetadata["target_msg_id"] = msgID // For debugging
		}

	case "batch":
		// User timeout/ban - provide user ID for frontend filtering
		if userID, ok := raw.EventData["target_user_id"].(string); ok {
			eventMetadata["target_user_id"] = userID
		}
		if username, ok := raw.EventData["target_username"].(string); ok {
			eventMetadata["target_username"] = username
		}
		// ban_duration survives a JSON round-trip over Redis Streams as float64
		// (gempir emits an int; the processor json.Unmarshals into a
		// map[string]interface{}, widening numbers to float64). Accept both —
		// otherwise the timeout duration is dropped and a timeout becomes
		// indistinguishable from a permanent ban downstream.
		switch d := raw.EventData["ban_duration"].(type) {
		case float64:
			eventMetadata["ban_duration"] = int(d)
		case int:
			eventMetadata["ban_duration"] = d
		}

	case "clear":
		// Full chat clear - no additional data needed
	}

	return &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   raw.OverlayID, // Set by router
		Platform:    raw.Platform,
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelName,
		Timestamp:   raw.Timestamp,
		Event: &models.EventInfo{
			Type:     "message_deletion",
			Metadata: eventMetadata,
		},
		User:     models.UserInfo{},      // Empty for deletion events
		Message:  models.MessageInfo{},   // Empty for deletion events
		Metadata: map[string]interface{}{}, // Empty metadata for deletions
	}
}
