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
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/classifier"
	"github.com/caesar/all-chat/services/message-processor/models"
)

// ytEmoteEntry mirrors yt_emote_cache.EmoteEntry for JSON decoding.
// Duplicated here to avoid coupling services via shared module.
type ytEmoteEntry struct {
	Code string `json:"code"`
	URL  string `json:"url"`
	ID   string `json:"id"`
}

// findAllPositions returns all [start, end] byte positions of substr in s.
// Both start and end are inclusive indices (matching the Twitch IRC position convention
// and the expectation of the frontend renderMessage renderer).
func findAllPositions(s, substr string) [][]int {
	if substr == "" {
		return nil
	}
	var positions [][]int
	offset := 0
	for {
		idx := strings.Index(s[offset:], substr)
		if idx == -1 {
			break
		}
		start := offset + idx
		end := start + len(substr) - 1 // inclusive end
		positions = append(positions, []int{start, end})
		offset = start + len(substr)
	}
	return positions
}

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
			Emotes: n.extractYTEmotes(raw),
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

	// Owner (channel owner/broadcaster) - YouTube uses Material Icons "stars" (star in circle), gold
	if tags["is_owner"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "owner",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%23FFD600'%3E%3Cpath d='M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zm4.24 16L12 15.45 7.77 18l1.12-4.81-3.73-3.23 4.92-.42L12 5l1.92 4.53 4.92.42-3.73 3.23L16.23 18z'/%3E%3C/svg%3E",
		})
	}

	// Member badge: prefer real image URL from InnerTube, fall back to SVG for old listener
	if tags["badge_member_url"] != "" {
		tooltip := tags["badge_member_tooltip"]
		if tooltip == "" {
			tooltip = "Member"
		}
		badges = append(badges, models.Badge{
			Name:    "member",
			Version: tooltip,
			IconURL: tags["badge_member_url"],
		})
	} else if tags["is_sponsor"] == "true" {
		// Backward compatibility: old youtube-listener sets is_sponsor without badge_member_url
		badges = append(badges, models.Badge{
			Name:    "member",
			Version: "Member",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' fill='%230F9D58'%3E%3Cpath d='M8 1l2 5h5l-4 3.5 1.5 5.5-4.5-3.5-4.5 3.5 1.5-5.5-4-3.5h5z'/%3E%3C/svg%3E",
		})
	}

	// Moderator - YouTube uses Material Icons "build" (wrench), blue
	if tags["is_moderator"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "moderator",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%235E84F1'%3E%3Cpath d='M22.7 19l-9.1-9.1c.9-2.3.4-5-1.5-6.9-2-2-5-2.4-7.4-1.3L9 6 6 9 1.6 4.7C.4 7.1.9 10.1 2.9 12.1c1.9 1.9 4.6 2.4 6.9 1.5l9.1 9.1c.4.4 1 .4 1.4 0l2.3-2.3c.5-.4.5-1.1.1-1.4z'/%3E%3C/svg%3E",
		})
	}

	// Verified - YouTube uses Material Icons "check_circle", gray
	if tags["is_verified"] == "true" {
		badges = append(badges, models.Badge{
			Name:    "verified",
			Version: "1",
			IconURL: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='%23909090'%3E%3Cpath d='M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z'/%3E%3C/svg%3E",
		})
	}

	return badges
}

// extractYTEmotes parses the emote_data tag from an InnerTube message into Emote entries.
// Returns empty slice when tag is absent, empty, or invalid JSON.
func (n *YouTubeNormalizer) extractYTEmotes(raw *models.RawChatMessage) []models.Emote {
	emoteDataJSON, ok := raw.Tags["emote_data"]
	if !ok || emoteDataJSON == "" {
		return []models.Emote{}
	}
	var entries []ytEmoteEntry
	if err := json.Unmarshal([]byte(emoteDataJSON), &entries); err != nil {
		// Invalid JSON from tag — graceful degradation, no error propagated
		return []models.Emote{}
	}
	emotes := make([]models.Emote, 0, len(entries))
	for _, e := range entries {
		emotes = append(emotes, models.Emote{
			Code:      e.Code,
			Provider:  "youtube",
			URL:       e.URL,
			Positions: findAllPositions(raw.Text, e.Code),
		})
	}
	return emotes
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

// NormalizeEvent converts a RawChatMessage with event data to UnifiedChatMessage with EventInfo
func (n *YouTubeNormalizer) NormalizeEvent(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "youtube" {
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
	case "super_chat":
		amountMicros := int64(0)
		if a, ok := raw.EventData["amount_micros"].(int64); ok {
			amountMicros = a
		} else if a, ok := raw.EventData["amount_micros"].(float64); ok {
			amountMicros = int64(a)
		}
		currency := "USD"
		if c, ok := raw.EventData["currency"].(string); ok {
			currency = c
		}
		displayText := "$0.00"
		if d, ok := raw.EventData["amount_display"].(string); ok {
			displayText = d
		}

		eventValue = &models.EventValue{
			Amount:      float64(amountMicros),
			Currency:    currency,
			DisplayText: displayText,
		}

	case "super_sticker":
		amountMicros := int64(0)
		if a, ok := raw.EventData["amount_micros"].(int64); ok {
			amountMicros = a
		} else if a, ok := raw.EventData["amount_micros"].(float64); ok {
			amountMicros = int64(a)
		}
		currency := "USD"
		if c, ok := raw.EventData["currency"].(string); ok {
			currency = c
		}
		displayText := "$0.00"
		if d, ok := raw.EventData["amount_display"].(string); ok {
			displayText = d
		}

		eventValue = &models.EventValue{
			Amount:      float64(amountMicros),
			Currency:    currency,
			DisplayText: displayText,
		}

	case "new_sponsor":
		eventValue = &models.EventValue{
			Amount:      1,
			Currency:    "membership",
			DisplayText: "New member",
		}

	case "member_milestone":
		months := 0
		if m, ok := raw.EventData["member_months"].(int); ok {
			months = m
		} else if m, ok := raw.EventData["member_months"].(float64); ok {
			months = int(m)
		}

		eventValue = &models.EventValue{
			Amount:      float64(months),
			Currency:    "months",
			DisplayText: fmt.Sprintf("%d month milestone", months),
		}

	case "membership_gift":
		giftCount := 0
		if g, ok := raw.EventData["gift_count"].(int); ok {
			giftCount = g
		} else if g, ok := raw.EventData["gift_count"].(float64); ok {
			giftCount = int(g)
		}

		eventValue = &models.EventValue{
			Amount:      float64(giftCount),
			Currency:    "gifts",
			DisplayText: fmt.Sprintf("%d gift memberships", giftCount),
		}

	case "gift_received":
		eventValue = &models.EventValue{
			Amount:      1,
			Currency:    "gift",
			DisplayText: "Received gift membership",
		}
	}

	// Classify event tier and duration
	tier, duration := classifier.ClassifyEvent("youtube", raw.EventType, eventValue)

	// Create EventInfo
	eventInfo := &models.EventInfo{
		Type:     raw.EventType,
		Tier:     tier,
		Value:    eventValue,
		Duration: duration,
		IsUpdate: false, // YouTube events don't have updates
		Metadata: raw.EventData,
	}

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "youtube",
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
