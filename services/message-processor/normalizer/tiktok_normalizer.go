package normalizer

import (
	"fmt"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// TikTokNormalizer normalizes TikTok raw messages to unified format
// Note: Uses data from unofficial TikTok-Live-Connector library
type TikTokNormalizer struct{}

// NewTikTokNormalizer creates a new TikTok normalizer
func NewTikTokNormalizer() *TikTokNormalizer {
	return &TikTokNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *TikTokNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "tiktok" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	// Extract user info from tags
	userInfo := n.extractUserInfo(raw)

	// TikTok native emotes - currently not extracted by unofficial library
	// This will be populated by the emote enricher
	emotes := make([]models.Emote, 0)

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "tiktok",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID, // TikTok username
		User:        userInfo,
		Message: models.MessageInfo{
			Text:   raw.Text,
			Emotes: emotes,
		},
		Timestamp: raw.Timestamp,
		Metadata:  n.extractMetadata(raw),
	}

	return unified, nil
}

// extractUserInfo extracts user information from tags
func (n *TikTokNormalizer) extractUserInfo(raw *models.RawChatMessage) models.UserInfo {
	tags := raw.Tags

	// Extract badges (TikTok has badge levels)
	badges := make([]models.Badge, 0)

	// Add follower badge if applicable
	if isFollower := tags["is_follower"]; isFollower == "true" {
		badges = append(badges, models.Badge{
			Name:    "follower",
			Version: "1",
			IconURL: "", // TikTok doesn't provide badge URLs via unofficial lib
		})
	}

	// Add subscriber badge if applicable
	if isSubscriber := tags["is_subscriber"]; isSubscriber == "true" {
		badges = append(badges, models.Badge{
			Name:    "subscriber",
			Version: "1",
			IconURL: "", // TikTok doesn't provide badge URLs via unofficial lib
		})
	}

	// Add badge level if > 0
	if badgeLevel := tags["badge_level"]; badgeLevel != "" && badgeLevel != "0" {
		badges = append(badges, models.Badge{
			Name:    fmt.Sprintf("level_%s", badgeLevel),
			Version: badgeLevel,
			IconURL: "", // TikTok doesn't provide badge URLs via unofficial lib
		})
	}

	// Get user info
	displayName := raw.Username
	userUniqueID := tags["user_unique_id"]
	profilePictureURL := tags["profile_picture_url"]

	// Use unique_id as username if available (e.g., @username)
	username := userUniqueID
	if username == "" {
		username = raw.UserID
	}

	// Remove @ prefix if present
	username = strings.TrimPrefix(username, "@")

	return models.UserInfo{
		ID:          raw.UserID,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   profilePictureURL,
		Badges:      badges,
		Color:       "#FE2C55", // TikTok brand color as default
	}
}

// extractMetadata extracts platform-specific metadata
func (n *TikTokNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	tags := raw.Tags

	metadata := make(map[string]interface{})
	metadata["is_subscriber"] = tags["is_subscriber"] == "true"
	metadata["is_moderator"] = false // TikTok doesn't have moderator concept in unofficial lib
	metadata["bits"] = 0             // TikTok doesn't use bits
	metadata["super_chat_amount"] = 0 // SuperChatAmount would be for TikTok gifts
	metadata["raw_tags"] = tags

	return metadata
}
