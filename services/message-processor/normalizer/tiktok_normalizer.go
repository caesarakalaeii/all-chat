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

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/classifier"
	"github.com/caesar/all-chat/services/message-processor/models"
)

// ttEmoteEntry mirrors the emote_data tag entries the tiktok-listener
// serializes from a chat payload's native emotes (the same shape as the
// YouTube listener's EmoteEntry). Duplicated here to avoid coupling services
// via a shared module.
type ttEmoteEntry struct {
	Code string `json:"code"` // visible token in the message text, e.g. "[laughcry]"
	URL  string `json:"url"`
	ID   string `json:"id"`
}

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

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	// Extract user info from tags
	userInfo := n.extractUserInfo(raw)

	// TikTok native emotes arrive as the emote_data tag the tiktok-listener
	// serializes from the chat payload's emotes array. They are platform-native
	// (like Twitch native emotes), so extraction happens here rather than in the
	// emote enricher, which only resolves third-party emote sets (7TV/BTTV/FFZ/
	// Twitch global) and would leave TikTok tokens as literal text.
	emotes := n.extractTTEmotes(raw)

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

// extractTTEmotes parses the emote_data tag from a TikTok chat message into
// Emote entries. Returns empty slice when tag is absent, empty, or invalid JSON.
func (n *TikTokNormalizer) extractTTEmotes(raw *models.RawChatMessage) []models.Emote {
	// Listener serializes one entry per emote occurrence in wire order; the
	// entries carry no offsets, so spans are found by substring search. That
	// keeps Positions byte-exact regardless of the unit (bytes, runes or UTF-16
	// code units) TikTok's emote start index counts, which the listener had to
	// resolve when it recovered each visible token from the comment text.
	emoteDataJSON, ok := raw.Tags["emote_data"]
	if !ok || emoteDataJSON == "" {
		return []models.Emote{}
	}
	var entries []ttEmoteEntry
	if err := json.Unmarshal([]byte(emoteDataJSON), &entries); err != nil {
		// Invalid JSON from tag — graceful degradation, no error propagated
		return []models.Emote{}
	}
	emotes := make([]models.Emote, 0, len(entries))
	for _, e := range entries {
		emotes = append(emotes, models.Emote{
			Code:      e.Code,
			Provider:  "tiktok",
			URL:       e.URL,
			Positions: findAllPositions(raw.Text, e.Code),
		})
	}
	return emotes
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
		// No platform-native color: TikTok's unofficial library exposes no
		// per-user color, and the old hardcoded brand pink (#FE2C55) made every
		// TikTok viewer identical. Leaving this empty lets the viewer-badge
		// enricher assign each viewer a deterministic auto-color instead.
		Color: "",
	}
}

// extractMetadata extracts platform-specific metadata
func (n *TikTokNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	tags := raw.Tags

	metadata := make(map[string]interface{})
	metadata["is_subscriber"] = tags["is_subscriber"] == "true"
	metadata["is_moderator"] = false  // TikTok doesn't have moderator concept in unofficial lib
	metadata["bits"] = 0              // TikTok doesn't use bits
	metadata["super_chat_amount"] = 0 // SuperChatAmount would be for TikTok gifts
	metadata["raw_tags"] = tags

	return metadata
}

// NormalizeEvent converts a RawChatMessage with event data to UnifiedChatMessage with EventInfo
func (n *TikTokNormalizer) NormalizeEvent(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "tiktok" {
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
	case "gift":
		giftName := "gift"
		if n, ok := raw.EventData["gift_name"].(string); ok {
			giftName = n
		}
		diamondCount := 0
		if d, ok := raw.EventData["diamond_count"].(int); ok {
			diamondCount = d
		} else if d, ok := raw.EventData["diamond_count"].(float64); ok {
			diamondCount = int(d)
		}

		eventValue = &models.EventValue{
			Amount:      float64(diamondCount),
			Currency:    "diamonds",
			DisplayText: fmt.Sprintf("%s (%d diamonds)", giftName, diamondCount),
		}

	case "like_aggregate":
		likeCount := 0
		if l, ok := raw.EventData["like_count"].(int); ok {
			likeCount = l
		} else if l, ok := raw.EventData["like_count"].(float64); ok {
			likeCount = int(l)
		}

		eventValue = &models.EventValue{
			Amount:      float64(likeCount),
			Currency:    "likes",
			DisplayText: fmt.Sprintf("%d like%s", likeCount, pluralize(likeCount)),
		}

	case "follow":
		eventValue = &models.EventValue{
			Amount:      1,
			Currency:    "follow",
			DisplayText: "New follower",
		}

	case "share":
		eventValue = &models.EventValue{
			Amount:      1,
			Currency:    "share",
			DisplayText: "Shared stream",
		}

	case "treasure_chest":
		coins := 0
		if c, ok := raw.EventData["coins"].(int); ok {
			coins = c
		} else if c, ok := raw.EventData["coins"].(float64); ok {
			coins = int(c)
		}

		// can_open (how many viewers may claim the chest) rides along in EventInfo.Metadata,
		// which is raw.EventData verbatim - no dedicated EventValue field for it.
		eventValue = &models.EventValue{
			Amount:      float64(coins),
			Currency:    "coins",
			DisplayText: fmt.Sprintf("%d coins", coins),
		}
	}

	// Classify event tier and duration
	tier, duration := classifier.ClassifyEvent("tiktok", raw.EventType, eventValue)

	// Extract aggregation info (for like updates)
	aggregationID := ""
	isUpdate := false
	if raw.EventType == "like_aggregate" {
		if id, ok := raw.EventData["aggregation_id"].(string); ok {
			aggregationID = id
		}
		if upd, ok := raw.EventData["is_update"].(bool); ok {
			isUpdate = upd
		}
	}

	// Create EventInfo
	eventInfo := &models.EventInfo{
		Type:          raw.EventType,
		Tier:          tier,
		Value:         eventValue,
		Duration:      duration,
		AggregationID: aggregationID,
		IsUpdate:      isUpdate, // CRITICAL: True for TikTok like updates
		Metadata:      raw.EventData,
	}

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "tiktok",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID,
		User:        userInfo,
		Message: models.MessageInfo{
			Text:   raw.Text,
			Emotes: []models.Emote{}, // Events don't have emotes
		},
		Timestamp: raw.Timestamp,
		Metadata:  n.extractMetadata(raw),
		Event:     eventInfo, // Add event info
	}

	return unified, nil
}

// pluralize returns "s" if count != 1, otherwise ""
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
