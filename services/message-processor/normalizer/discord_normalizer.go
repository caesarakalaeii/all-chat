package normalizer

import (
	"fmt"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// DiscordNormalizer normalizes Discord chat messages to unified format
type DiscordNormalizer struct{}

// NewDiscordNormalizer creates a new Discord message normalizer
func NewDiscordNormalizer() *DiscordNormalizer {
	return &DiscordNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *DiscordNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "discord" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	displayName := raw.Tags["member_nick"]
	if displayName == "" {
		displayName = raw.Username
	}

	color := raw.Tags["role_color"]
	if color == "#000000" {
		color = ""
	}

	badges := extractDiscordBadges(raw.Tags["badges"])

	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "discord",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          raw.UserID,
			Username:    raw.Username,
			DisplayName: displayName,
			Color:       color,
			Badges:      badges,
			AvatarURL:   raw.Tags["avatar_url"],
		},
		Message: models.MessageInfo{
			Text:   raw.Text,
			Emotes: []models.Emote{},
		},
		Timestamp: raw.Timestamp,
		Metadata:  map[string]interface{}{},
	}

	return unified, nil
}

// extractDiscordBadges parses a comma-separated badge tag string into a Badge slice.
// Known badge values: "moderator", "admin", "vip". All badges get Version "1".
func extractDiscordBadges(badgeTag string) []models.Badge {
	badges := make([]models.Badge, 0)
	if badgeTag == "" {
		return badges
	}
	for _, name := range strings.Split(badgeTag, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			badges = append(badges, models.Badge{Name: name, Version: "1"})
		}
	}
	return badges
}
