package normalizer

import (
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// KickNormalizer normalizes Kick chat messages to unified format
type KickNormalizer struct{}

// NewKickNormalizer creates a new Kick message normalizer
func NewKickNormalizer() *KickNormalizer {
	return &KickNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *KickNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "kick" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	// For Kick, the raw.Text might contain JSON data
	// Try to parse it as a Kick message structure
	// The Kick listener should have stored the message in the Text field as JSON

	// Parse timestamp
	timestamp := raw.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// Extract badges from tags
	badges := n.extractBadges(raw)

	// Extract metadata
	metadata := n.extractMetadata(raw)

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "kick",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID, // Use channel ID as name
		User: models.UserInfo{
			ID:          raw.UserID,
			Username:    raw.Username,
			DisplayName: raw.Username,
			AvatarURL:   "", // Kick doesn't provide avatar in raw messages
			Badges:      badges,
			Color:       raw.Tags["color"],
		},
		Message: models.MessageInfo{
			Text:   raw.Text,
			Emotes: []models.Emote{}, // Emotes will be enriched by emote enricher
		},
		Timestamp: timestamp,
		Metadata:  metadata,
	}

	return unified, nil
}

// extractBadges extracts badges from tags
func (n *KickNormalizer) extractBadges(raw *models.RawChatMessage) []models.Badge {
	badges := make([]models.Badge, 0)

	// Kick badges might be in tags as comma-separated values
	if badgesStr, ok := raw.Tags["badges"]; ok && badgesStr != "" {
		// Expected format: "subscriber,moderator,vip"
		badgeNames := splitAndTrim(badgesStr, ",")
		for _, name := range badgeNames {
			badges = append(badges, models.Badge{
				Name:    name,
				Version: "1",
				IconURL: "", // Kick badge icons would need to be fetched separately
			})
		}
	}

	return badges
}

// extractMetadata extracts metadata from tags
func (n *KickNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Check for subscriber status
	if sub, ok := raw.Tags["subscriber"]; ok && sub == "1" {
		metadata["is_subscriber"] = true
	} else {
		metadata["is_subscriber"] = false
	}

	// Check for moderator status
	if mod, ok := raw.Tags["moderator"]; ok && mod == "1" {
		metadata["is_moderator"] = true
	} else {
		metadata["is_moderator"] = false
	}

	// Check for VIP status
	if vip, ok := raw.Tags["vip"]; ok && vip == "1" {
		metadata["is_vip"] = true
	}

	// Check for founder status
	if founder, ok := raw.Tags["founder"]; ok && founder == "1" {
		metadata["is_founder"] = true
	}

	// Message type
	if msgType, ok := raw.Tags["message_type"]; ok {
		metadata["message_type"] = msgType
	}

	// Chatroom ID
	if chatroomID, ok := raw.Tags["chatroom_id"]; ok {
		metadata["chatroom_id"] = chatroomID
	}

	return metadata
}

// splitAndTrim splits a string by delimiter and trims whitespace
func splitAndTrim(s, delimiter string) []string {
	if s == "" {
		return []string{}
	}

	parts := make([]string, 0)
	for _, part := range splitString(s, delimiter) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString splits a string by delimiter
func splitString(s, delimiter string) []string {
	result := make([]string, 0)
	current := ""

	for _, char := range s {
		if string(char) == delimiter {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
