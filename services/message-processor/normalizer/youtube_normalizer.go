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

	// Owner (channel owner/broadcaster)
	if tags["is_owner"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "owner",
			Version: "1",
			IconURL: "https://www.youtube.com/s/desktop/d743f786/img/favicon_96x96.png", // YouTube icon as placeholder
		})
	}

	// Sponsor (channel member)
	if tags["is_sponsor"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "member",
			Version: "1",
			IconURL: "https://www.youtube.com/s/desktop/d743f786/img/favicon_96x96.png",
		})
	}

	// Moderator
	if tags["is_moderator"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "moderator",
			Version: "1",
			IconURL: "https://www.youtube.com/s/desktop/d743f786/img/favicon_96x96.png",
		})
	}

	// Verified
	if tags["is_verified"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "verified",
			Version: "1",
			IconURL: "https://www.youtube.com/s/desktop/d743f786/img/favicon_96x96.png",
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
