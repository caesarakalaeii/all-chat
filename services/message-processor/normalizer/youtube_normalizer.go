package normalizer

import (
	"fmt"
	"strconv"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// YouTubeNormalizer normalizes YouTube raw messages to unified format
type YouTubeNormalizer struct{}

// NewYouTubeNormalizer creates a new YouTube normalizer
func NewYouTubeNormalizer() *YouTubeNormalizer {
	return &YouTubeNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *YouTubeNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "youtube" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	// Extract user info from tags
	userInfo := n.extractUserInfo(raw)

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "youtube",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.Tags["display_name"],
		User:        userInfo,
		Message: models.MessageInfo{
			Text:   raw.Text,
			Emotes: []models.Emote{}, // Will be enriched with third-party emotes
		},
		Timestamp: raw.Timestamp,
		Metadata:  n.extractMetadata(raw),
	}

	return unified, nil
}

// extractUserInfo extracts user information from tags
func (n *YouTubeNormalizer) extractUserInfo(raw *models.RawChatMessage) models.UserInfo {
	tags := raw.Tags

	// Extract badges
	badges := n.extractBadges(tags)

	return models.UserInfo{
		ID:          raw.UserID,
		Username:    raw.Username,
		DisplayName: raw.Username,
		AvatarURL:   tags["profile_image"],
		Badges:      badges,
		Color:       "", // YouTube doesn't have user colors
	}
}

// extractBadges extracts YouTube badges from tags
func (n *YouTubeNormalizer) extractBadges(tags map[string]string) []models.Badge {
	badges := make([]models.Badge, 0)

	// Owner (channel owner/broadcaster) - Crown icon
	if tags["is_owner"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "owner",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' fill='%23FFD700'%3E%3Cpath d='M8 2l1.5 3h3.5l-2.8 2 1 3.2-3.2-2.2-3.2 2.2 1-3.2-2.8-2h3.5z'/%3E%3Cpath d='M2 13h12v2H2z' fill='%23FFA500'/%3E%3C/svg%3E",
		})
	}

	// Sponsor (channel member) - Star icon
	if tags["is_sponsor"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "member",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' fill='%2300FF00'%3E%3Cpath d='M8 1l2 5h5l-4 3.5 1.5 5.5-4.5-3.5-4.5 3.5 1.5-5.5-4-3.5h5z'/%3E%3C/svg%3E",
		})
	}

	// Moderator - Shield with wrench icon
	if tags["is_moderator"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "moderator",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' fill='%235E84F1'%3E%3Cpath d='M8 1L3 3v4c0 3.5 2 6 5 8 3-2 5-4.5 5-8V3z'/%3E%3Cpath d='M6 6h1v4H6zm3 0h1v4H9z' fill='white'/%3E%3C/svg%3E",
		})
	}

	// Verified - Checkmark icon
	if tags["is_verified"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "verified",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' fill='%2300A6ED'%3E%3Ccircle cx='8' cy='8' r='7'/%3E%3Cpath d='M6 8l2 2 4-4' stroke='white' stroke-width='2' fill='none'/%3E%3C/svg%3E",
		})
	}

	return badges
}

// extractMetadata extracts additional metadata from tags
func (n *YouTubeNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	tags := raw.Tags

	// Boolean flags
	metadata["is_verified"] = tags["is_verified"] == "true"
	metadata["is_owner"] = tags["is_owner"] == "true"
	metadata["is_sponsor"] = tags["is_sponsor"] == "true"
	metadata["is_moderator"] = tags["is_moderator"] == "true"

	// YouTube-specific: Super Chat and Super Sticker amounts (in micros)
	superChat := parseInt64OrZero(tags["super_chat"])
	superSticker := parseInt64OrZero(tags["super_sticker"])

	metadata["super_chat_amount"] = superChat
	metadata["super_sticker_amount"] = superSticker

	// If Super Chat or Super Sticker, include display strings
	if superChat > 0 {
		metadata["super_chat_currency"] = tags["super_chat_currency"]
		metadata["super_chat_display"] = tags["super_chat_display"]
	}

	if superSticker > 0 {
		metadata["super_sticker_currency"] = tags["super_sticker_currency"]
		metadata["super_sticker_display"] = tags["super_sticker_display"]
		metadata["super_sticker_tier"] = tags["super_sticker_tier"]
	}

	// Not applicable for YouTube
	metadata["bits"] = 0
	metadata["is_subscriber"] = false // YouTube uses "is_sponsor" instead
	metadata["is_turbo"] = false      // Twitch-only

	return metadata
}

// parseInt64OrZero parses a string to int64, returns 0 on error
func parseInt64OrZero(s string) int64 {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
