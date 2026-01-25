package normalizer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/classifier"
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

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
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

	// Extract badges with URLs
	badges := make([]models.Badge, 0)
	if badgesStr, ok := tags["badges"]; ok && badgesStr != "" {
		// Format: "subscriber/12,moderator/1"
		badgePairs := strings.Split(badgesStr, ",")
		for _, pair := range badgePairs {
			parts := strings.Split(pair, "/")
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]

				// Don't set placeholder URLs - let the badge enricher populate them
				// The old CDN format https://static-cdn.jtvnw.net/badges/v1/{name}/{version}/1 returns 404
				badges = append(badges, models.Badge{
					Name:    name,
					Version: version,
					IconURL: "", // Will be enriched by badge enricher
				})
			}
		}
	}

	// Extract source badges for shared chat (if present)
	sourceBadges := make([]models.Badge, 0)
	if sourceBadgesStr, ok := tags["source-badges"]; ok && sourceBadgesStr != "" {
		// Format: "subscriber/12,moderator/1"
		badgePairs := strings.Split(sourceBadgesStr, ",")
		for _, pair := range badgePairs {
			parts := strings.Split(pair, "/")
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]

				sourceBadges = append(sourceBadges, models.Badge{
					Name:    name,
					Version: version,
					IconURL: "", // Will be enriched by badge enricher using source channel
				})
			}
		}
	}

	// Get display name (fallback to username)
	displayName := tags["display-name"]
	if displayName == "" {
		displayName = raw.Username
	}

	// Extract source user ID for shared chat
	sourceUserID := tags["source-id"]

	return models.UserInfo{
		ID:           raw.UserID,
		Username:     raw.Username,
		DisplayName:  displayName,
		AvatarURL:    "", // Will be enriched by avatar enricher
		Badges:       badges,
		Color:        tags["color"],
		SourceBadges: sourceBadges,
		SourceUserID: sourceUserID,
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

	// Shared Chat detection and metadata
	sourceRoomID := tags["source-room-id"]
	if sourceRoomID != "" {
		metadata["is_shared_chat"] = true
		metadata["source_room_id"] = sourceRoomID
		// Note: source channel name would need to be resolved via Twitch API
		// For now, we just track the room ID
	} else {
		metadata["is_shared_chat"] = false
	}

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

// NormalizeEvent converts a RawChatMessage with event data to UnifiedChatMessage with EventInfo
func (n *TwitchNormalizer) NormalizeEvent(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "twitch" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if raw.EventType == "" {
		return nil, fmt.Errorf("missing event type")
	}

	// Extract user info
	userInfo := n.extractUserInfo(raw)

	// Build EventValue from EventData
	var eventValue *models.EventValue

	switch raw.EventType {
	case "subscription", "resubscription":
		// Extract tier and months
		tier := "1000" // Default to Tier 1
		if t, ok := raw.EventData["tier"].(string); ok {
			tier = t
		}
		months := 0
		if m, ok := raw.EventData["months"].(int); ok {
			months = m
		} else if m, ok := raw.EventData["months"].(float64); ok {
			months = int(m)
		}

		tierName := getTierName(tier)
		eventValue = &models.EventValue{
			Amount:      float64(months),
			Currency:    "months",
			DisplayText: fmt.Sprintf("%s - %d months", tierName, months),
		}

	case "gift_subscription":
		tier := "1000"
		if t, ok := raw.EventData["tier"].(string); ok {
			tier = t
		}
		recipient := "someone"
		if r, ok := raw.EventData["recipient_name"].(string); ok {
			recipient = r
		}

		tierName := getTierName(tier)
		eventValue = &models.EventValue{
			Amount:      1,
			Currency:    "gift",
			DisplayText: fmt.Sprintf("Gifted %s sub to %s", tierName, recipient),
		}

	case "mystery_gift":
		giftCount := 0
		if g, ok := raw.EventData["gift_count"].(int); ok {
			giftCount = g
		} else if g, ok := raw.EventData["gift_count"].(float64); ok {
			giftCount = int(g)
		}

		eventValue = &models.EventValue{
			Amount:      float64(giftCount),
			Currency:    "gifts",
			DisplayText: fmt.Sprintf("%d gift subs", giftCount),
		}

	case "raid":
		viewerCount := 0
		if v, ok := raw.EventData["viewer_count"].(int); ok {
			viewerCount = v
		} else if v, ok := raw.EventData["viewer_count"].(float64); ok {
			viewerCount = int(v)
		}

		eventValue = &models.EventValue{
			Amount:      float64(viewerCount),
			Currency:    "viewers",
			DisplayText: fmt.Sprintf("%d viewers", viewerCount),
		}

	case "bits":
		badgeTier := 0
		if b, ok := raw.EventData["badge_tier"].(int); ok {
			badgeTier = b
		} else if b, ok := raw.EventData["badge_tier"].(float64); ok {
			badgeTier = int(b)
		}

		eventValue = &models.EventValue{
			Amount:      float64(badgeTier),
			Currency:    "bits",
			DisplayText: fmt.Sprintf("%d bits", badgeTier),
		}

	case "channel_points":
		cost := 0
		if c, ok := raw.EventData["reward_cost"].(int); ok {
			cost = c
		} else if c, ok := raw.EventData["reward_cost"].(float64); ok {
			cost = int(c)
		}
		title := "Reward"
		if t, ok := raw.EventData["reward_title"].(string); ok {
			title = t
		}

		eventValue = &models.EventValue{
			Amount:      float64(cost),
			Currency:    "points",
			DisplayText: fmt.Sprintf("%s (%d points)", title, cost),
		}
	}

	// Classify event tier and duration
	tier, duration := classifier.ClassifyEvent("twitch", raw.EventType, eventValue)

	// Create EventInfo
	eventInfo := &models.EventInfo{
		Type:     raw.EventType,
		Tier:     tier,
		Value:    eventValue,
		Duration: duration,
		IsUpdate: false, // Twitch events don't have updates
		Metadata: raw.EventData,
	}

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID,
		User:        userInfo,
		Message: models.MessageInfo{
			Text:   raw.Text, // System message from Twitch
			Emotes: []models.Emote{}, // Events don't have emotes
		},
		Timestamp: raw.Timestamp,
		Metadata:  n.extractMetadata(raw),
		Event:     eventInfo, // Add event info
	}

	return unified, nil
}

// getTierName converts Twitch subscription tier to human-readable name
func getTierName(tier string) string {
	switch tier {
	case "1000":
		return "Tier 1"
	case "2000":
		return "Tier 2"
	case "3000":
		return "Tier 3"
	case "Prime":
		return "Prime"
	default:
		return tier
	}
}
