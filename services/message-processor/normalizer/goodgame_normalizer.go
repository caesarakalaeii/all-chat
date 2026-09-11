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
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// GoodGameNormalizer normalizes GoodGame.ru chat messages to the unified
// format. Input is the raw "message" frame payload from the chat WebSocket,
// as published by goodgame-listener to chat:raw (raw_message field).
type GoodGameNormalizer struct{}

// NewGoodGameNormalizer creates a new GoodGame message normalizer.
func NewGoodGameNormalizer() *GoodGameNormalizer {
	return &GoodGameNormalizer{}
}

func (n *GoodGameNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "goodgame" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	var msg *goodgameChatMessage
	if len(raw.RawMessage) > 0 {
		if parsed, err := parseGoodgameMessage(raw.RawMessage); err == nil {
			msg = parsed
		}
	}

	timestamp := raw.Timestamp
	if msg != nil && msg.Timestamp.String() != "" {
		if unix, err := msg.Timestamp.Int64(); err == nil && unix > 0 {
			timestamp = time.Unix(unix, 0).UTC()
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	messageID := raw.MessageID
	if messageID == "" && msg != nil {
		messageID = msg.MessageID.String()
	}
	if messageID == "" {
		messageID = "goodgame-" + strconv.FormatInt(timestamp.UnixNano(), 10)
	}

	userID := raw.UserID
	if userID == "" && msg != nil {
		userID = msg.UserID.String()
	}

	username := raw.Username
	if username == "" && msg != nil {
		username = msg.UserName
	}

	color := raw.Tags["color"]
	if color == "" && msg != nil {
		// GoodGame sends colour names ("gold", "simple", "none"); only pass
		// through actual hex values, named tiers are not a renderable colour.
		if isHexColor(msg.Color) {
			color = msg.Color
		}
	}

	unified := &models.UnifiedChatMessage{
		ID:          messageID,
		OverlayID:   overlayID,
		Platform:    "goodgame",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          userID,
			Username:    username,
			DisplayName: username,
			Badges:      extractGoodgameBadges(raw, msg),
			Color:       color,
		},
		Message: models.MessageInfo{
			Text:   raw.Text,
			Emotes: []models.Emote{},
		},
		Timestamp: timestamp,
		// Tags pass through for downstream consumers (premium tier, rights,
		// mobile flag) — snake_case keys as the listener publishes them.
		Metadata: metadataFromTags(raw.Tags),
	}

	return unified, nil
}

// goodgameChatMessage mirrors the GoodGame "message" frame payload. The
// listener may hand the raw JSON through; shapes grounded in the live
// endpoint capture and GoodGame/API Chat/protocol.md.
type goodgameChatMessage struct {
	ChannelID  json.Number `json:"channel_id"`
	UserID     json.Number `json:"user_id"`
	UserName   string      `json:"user_name"`
	UserRights json.Number `json:"user_rights"`
	Premium    json.Number `json:"premium"`
	Color      string      `json:"color"`
	Icon       string      `json:"icon"`
	Role       string      `json:"role"`
	Mobile     json.Number `json:"mobile"`
	MessageID  json.Number `json:"message_id"`
	Timestamp  json.Number `json:"timestamp"`
	Text       string      `json:"text"`
}

func parseGoodgameMessage(data json.RawMessage) (*goodgameChatMessage, error) {
	var msg goodgameChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// extractGoodgameBadges maps protocol fields to badges. GoodGame has no badge
// array in its message frames — rights/premium/staff/icon are the only
// signals, mapped defensively; empty is fine.
func extractGoodgameBadges(raw *models.RawChatMessage, msg *goodgameChatMessage) []models.Badge {
	var badges []models.Badge
	rights := valueFrom(raw, msg, "user_rights", "user_rights")
	premium := valueFrom(raw, msg, "premium", "premium")
	if msg != nil && msg.Role != "" {
		badges = append(badges, models.Badge{Name: msg.Role})
	} else if rights != "" && rights != "0" {
		// GoodGame rights ladder: 10 stream helper, 20 streamer, 30 moderator, 50 admin.
		badges = append(badges, models.Badge{Name: "rights:" + rights})
	}
	if premium != "" && premium != "0" {
		badges = append(badges, models.Badge{Name: "premium"})
	}
	return badges
}

// valueFrom prefers the tag key, falling back to the raw message field.
func valueFrom(raw *models.RawChatMessage, msg *goodgameChatMessage, tagKey, field string) string {
	if v := raw.Tags[tagKey]; v != "" {
		return v
	}
	if msg != nil {
		switch field {
		case "user_rights":
			return msg.UserRights.String()
		case "premium":
			return msg.Premium.String()
		}
	}
	return ""
}

// metadataFromTags carries the listener-produced tag map into metadata so
// premium/rights survive to the overlay. Always non-nil.
func metadataFromTags(tags map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{}, len(tags))
	for k, v := range tags {
		metadata[k] = v
	}
	return metadata
}

// isHexColor reports s as a #RGB or #RRGGBB literal.
func isHexColor(s string) bool {
	if len(s) != 7 && len(s) != 4 {
		return false
	}
	if s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
