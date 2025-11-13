package normalizer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// TwitchNormalizer normalizes Twitch raw messages to unified format
type TwitchNormalizer struct{}

// NewTwitchNormalizer creates a new Twitch normalizer
func NewTwitchNormalizer() *TwitchNormalizer {
	return &TwitchNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *TwitchNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "twitch" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	// Extract user info from tags
	userInfo := n.extractUserInfo(raw)

	// Extract Twitch native emotes from tags
	emotes := n.extractTwitchEmotes(raw)

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID, // Twitch uses channel name as ID
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
func (n *TwitchNormalizer) extractUserInfo(raw *models.RawChatMessage) models.UserInfo {
	tags := raw.Tags

	// Extract badges
	badges := make([]string, 0)
	if badgesStr, ok := tags["badges"]; ok && badgesStr != "" {
		// Format: "subscriber/12,moderator/1"
		badgePairs := strings.Split(badgesStr, ",")
		for _, pair := range badgePairs {
			parts := strings.Split(pair, "/")
			if len(parts) > 0 {
				badges = append(badges, parts[0])
			}
		}
	}

	// Get display name (fallback to username)
	displayName := tags["display-name"]
	if displayName == "" {
		displayName = raw.Username
	}

	return models.UserInfo{
		ID:          raw.UserID,
		Username:    raw.Username,
		DisplayName: displayName,
		AvatarURL:   "", // Twitch doesn't provide this in IRC
		Badges:      badges,
		Color:       tags["color"],
	}
}

// extractTwitchEmotes extracts native Twitch emotes from IRC tags
func (n *TwitchNormalizer) extractTwitchEmotes(raw *models.RawChatMessage) []models.Emote {
	emotesStr, ok := raw.Tags["emotes"]
	if !ok || emotesStr == "" {
		return []models.Emote{}
	}

	// Parse emotes tag format: "25:0-4,12-16/1902:6-10"
	// Format: emoteID:start-end,start-end/emoteID:start-end
	emotes := make([]models.Emote, 0)

	emoteParts := strings.Split(emotesStr, "/")
	for _, part := range emoteParts {
		// Split emote ID from positions
		idAndPos := strings.Split(part, ":")
		if len(idAndPos) != 2 {
			continue
		}

		emoteID := idAndPos[0]
		positionsStr := idAndPos[1]

		// Parse positions
		positions := make([][]int, 0)
		posPairs := strings.Split(positionsStr, ",")
		for _, posPair := range posPairs {
			startEnd := strings.Split(posPair, "-")
			if len(startEnd) != 2 {
				continue
			}

			start, err1 := strconv.Atoi(startEnd[0])
			end, err2 := strconv.Atoi(startEnd[1])
			if err1 == nil && err2 == nil {
				positions = append(positions, []int{start, end})
			}
		}

		if len(positions) > 0 {
			// Extract emote code from message text using first position
			// IRC positions are inclusive, so end position is already the last character
			code := ""
			if len(positions) > 0 && positions[0][0] < len(raw.Text) && positions[0][1]+1 <= len(raw.Text) {
				code = raw.Text[positions[0][0] : positions[0][1]+1]
			}

			emotes = append(emotes, models.Emote{
				Code:      code,
				Provider:  "twitch",
				URL:       fmt.Sprintf("https://static-cdn.jtvnw.net/emoticons/v2/%s/default/dark/1.0", emoteID),
				Positions: positions,
			})
		}
	}

	return emotes
}

// extractMetadata extracts additional metadata from tags
func (n *TwitchNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	tags := raw.Tags

	// Boolean flags
	metadata["is_subscriber"] = tags["subscriber"] == "1"
	metadata["is_moderator"] = tags["mod"] == "1"
	metadata["is_turbo"] = tags["turbo"] == "1"

	// Message ID
	if msgID, ok := tags["id"]; ok {
		metadata["twitch_message_id"] = msgID
	}

	// Room ID
	if roomID, ok := tags["room-id"]; ok {
		metadata["twitch_room_id"] = roomID
	}

	// Timestamp
	if tmiSentTs, ok := tags["tmi-sent-ts"]; ok {
		metadata["twitch_sent_ts"] = tmiSentTs
	}

	// Bits (not usually in regular messages, but just in case)
	metadata["bits"] = 0

	// Super chat (YouTube only, always 0 for Twitch)
	metadata["super_chat_amount"] = 0

	return metadata
}
