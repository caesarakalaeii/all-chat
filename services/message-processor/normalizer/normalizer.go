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
		if duration, ok := raw.EventData["ban_duration"].(int); ok {
			eventMetadata["ban_duration"] = duration
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
