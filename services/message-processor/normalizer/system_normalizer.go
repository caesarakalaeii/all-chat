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
