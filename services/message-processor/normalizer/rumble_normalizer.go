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

// RumbleNormalizer normalizes Rumble chat messages (SSE pop-up API shape,
// ADR-0061) to the unified format.
type RumbleNormalizer struct{}

// NewRumbleNormalizer creates a new Rumble message normalizer.
func NewRumbleNormalizer() *RumbleNormalizer {
	return &RumbleNormalizer{}
}

// rumbleRawMessage mirrors the listener's SSE message payload: the message
// itself plus the users array of the same batch, which carries the sender's
// username, colour, badges and avatar (fields the message entry omits).
type rumbleRawMessage struct {
	Message rumbleSSEMessage `json:"message"`
	Users   []rumbleSSEUser  `json:"users"`
}

type rumbleSSEMessage struct {
	ID     string `json:"id"`
	Time   string `json:"time"`
	UserID string `json:"user_id"`
	Text   string `json:"text"`
	Type   string `json:"type"`
	Rant   *struct {
		PriceCents int `json:"price_cents"`
		Duration   int `json:"duration"`
	} `json:"rant"`
	IsDeleted bool `json:"is_deleted"`
}

type rumbleSSEUser struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Image    string   `json:"image.1"`
	Color    string   `json:"color"`
	Badges   []string `json:"badges"`
}

// Normalize converts a RawChatMessage to UnifiedChatMessage.
func (n *RumbleNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "rumble" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	var rumbleMsg *rumbleRawMessage
	if len(raw.RawMessage) > 0 {
		if parsed, err := parseRumbleMessage(raw.RawMessage); err == nil {
			rumbleMsg = parsed
		}
	}

	timestamp := raw.Timestamp
	if timestamp.IsZero() && rumbleMsg != nil && rumbleMsg.Message.Time != "" {
		if parsed, err := time.Parse(time.RFC3339, rumbleMsg.Message.Time); err == nil {
			timestamp = parsed
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	text := raw.Text
	if text == "" && rumbleMsg != nil {
		text = rumbleMsg.Message.Text
	}

	messageID := raw.MessageID
	if messageID == "" && rumbleMsg != nil {
		messageID = rumbleMsg.Message.ID
	}
	if messageID == "" {
		messageID = fmt.Sprintf("rumble-%d", timestamp.UnixNano())
	}

	sender := rumbleUserFor(rumbleMsg, raw.UserID, raw.Tags)
	userID := raw.UserID
	if userID == "" && rumbleMsg != nil {
		userID = rumbleMsg.Message.UserID
	}
	if userID == "" && sender != nil {
		userID = sender.ID
	}
	username := raw.Username
	// Username resolution: the SSE message carries only the user id, so
	// unqualified id fallbacks must not leak through as a display name.
	if sender != nil && sender.Username != "" && (username == "" || username == userID) {
		username = sender.Username
	}
	color := ""
	avatar := ""
	var badges []models.Badge
	if sender != nil {
		color = sender.Color
		avatar = sender.Image
		badges = rumbleBadges(sender.Badges)
	}
	if tagColor := raw.Tags["color"]; tagColor != "" && color == "" {
		color = tagColor
	}

	metadata := n.extractMetadata(raw, rumbleMsg)

	unified := &models.UnifiedChatMessage{
		ID:          messageID,
		OverlayID:   overlayID,
		Platform:    "rumble",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          userID,
			Username:    username,
			DisplayName: username,
			AvatarURL:   avatar,
			Badges:      badges,
			Color:       color,
		},
		Message: models.MessageInfo{
			Text: text,
		},
		Timestamp: timestamp,
		Metadata:  metadata,
	}

	// Rants (donations) surface as an event with the amount, matching how
	// other platforms deliver monetary activity to the overlay.
	if rumbleMsg != nil && rumbleMsg.Message.Rant != nil && rumbleMsg.Message.Rant.PriceCents > 0 {
		cents := rumbleMsg.Message.Rant.PriceCents
		unified.Event = &models.EventInfo{
			Type: "donation",
			Value: &models.EventValue{
				Amount:      float64(cents) / 100,
				Currency:    "USD",
				DisplayText: fmt.Sprintf("$%.2f", float64(cents)/100),
			},
			Duration: rumbleMsg.Message.Rant.Duration,
			Metadata: map[string]interface{}{
				"rant": true,
			},
		}
	}

	return unified, nil
}

func (n *RumbleNormalizer) extractMetadata(raw *models.RawChatMessage, rumbleMsg *rumbleRawMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	if raw.Tags != nil {
		if chatroomID, ok := raw.Tags["chatroom_id"]; ok && chatroomID != "" {
			if numeric, err := strconv.Atoi(chatroomID); err == nil {
				metadata["chatroom_id"] = numeric
			} else {
				metadata["chatroom_id"] = chatroomID
			}
		}
		if msgType, ok := raw.Tags["message_type"]; ok && msgType != "" {
			metadata["message_type"] = msgType
		}
	}

	if rumbleMsg != nil {
		if metadata["message_type"] == nil && rumbleMsg.Message.Type != "" {
			metadata["message_type"] = rumbleMsg.Message.Type
		}
	}

	// Derived role flags from observed badge slugs. Unknown slugs are simply
	// badges; the set below covers the slugs Rumble's own client special-cases.
	badgeSet := make(map[string]struct{})
	if sender := rumbleUserFor(rumbleMsg, raw.UserID, raw.Tags); sender != nil {
		for _, b := range sender.Badges {
			badgeSet[b] = struct{}{}
		}
	}
	_, isSub := badgeSet["recurring_subscription"]
	_, isLocals := badgeSet["locals_supporter"]
	_, isAdmin := badgeSet["admin"]

	metadata["is_subscriber"] = isSub || isLocals
	metadata["is_moderator"] = isAdmin
	metadata["is_admin"] = isAdmin

	return metadata
}

// rumbleUserFor resolves the sender profile from the batch users array,
// matching on the message's user id. Falls back gracefully to nil.
func rumbleUserFor(rumbleMsg *rumbleRawMessage, rawUserID string, tags map[string]string) *rumbleSSEUser {
	if rumbleMsg == nil {
		return nil
	}
	userID := rumbleMsg.Message.UserID
	if userID == "" {
		userID = rawUserID
	}
	if userID == "" {
		if id, ok := tags["sender_user_id"]; ok {
			userID = id
		}
	}
	if userID == "" {
		return nil
	}
	for i := range rumbleMsg.Users {
		if rumbleMsg.Users[i].ID == userID {
			return &rumbleMsg.Users[i]
		}
	}
	return nil
}

// rumbleBadges converts badge slugs to unified badges. Rumble's icon catalog
// arrives per-chat via the init config, which the listener does not retain,
// so badges carry the slug only (IconURL empty is a supported render state).
func rumbleBadges(slugs []string) []models.Badge {
	if len(slugs) == 0 {
		return []models.Badge{}
	}
	badges := make([]models.Badge, 0, len(slugs))
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		badges = append(badges, models.Badge{
			Name:    slug,
			Version: "1",
		})
	}
	return badges
}

func parseRumbleMessage(raw json.RawMessage) (*rumbleRawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty rumble payload")
	}
	var msg rumbleRawMessage
	// The listener publishes the SSE message JSON; users arrive in the same
	// batch and are re-wrapped by the listener into this envelope. Accept a
	// bare message object too, so a future listener change cannot strand
	// normalization.
	if err := json.Unmarshal(raw, &msg); err == nil && msg.Message.ID != "" {
		return &msg, nil
	}
	var single rumbleSSEMessage
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, err
	}
	return &rumbleRawMessage{Message: single}, nil
}
