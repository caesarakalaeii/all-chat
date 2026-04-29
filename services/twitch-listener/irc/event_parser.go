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

package irc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/gempir/go-twitch-irc/v4"
	"github.com/google/uuid"
)

// ParseUserNotice converts a Twitch IRC USERNOTICE into a RawChatMessage with
// event fields. Returns (nil, nil) for events covered by twitch-eventsub-listener
// (subs, resubs, gifts, raids) — those are surfaced canonically via EventSub
// with richer data; emitting them from IRC too produces duplicate overlay
// entries and a bogus "0 months" tenure for new subs (#254).
func (p *Parser) ParseUserNotice(msg twitch.UserNoticeMessage) (*models.RawChatMessage, error) {
	if msg.Channel == "" {
		return nil, fmt.Errorf("missing channel")
	}

	// Extract msg-id to determine event type
	msgID := msg.MsgID
	if msgID == "" {
		return nil, fmt.Errorf("missing msg-id")
	}

	if isCoveredByEventSub(msgID) {
		return nil, nil
	}

	// Map msg-id to our event type taxonomy
	eventType := mapMsgIDToEventType(msgID)
	if eventType == "unknown" {
		return nil, fmt.Errorf("unknown msg-id: %s", msgID)
	}

	// Extract event-specific data from msg-param-* tags
	eventData := extractEventDataFromTags(msg.Tags, msgID)

	// Build tags map
	tags := make(map[string]string)
	tags["msg-id"] = msgID
	tags["system-msg"] = msg.SystemMsg // User-friendly message from Twitch

	// User info
	if msg.User.ID != "" {
		tags["user-id"] = msg.User.ID
	}
	if msg.User.DisplayName != "" {
		tags["display-name"] = msg.User.DisplayName
	}
	if msg.User.Name != "" {
		tags["login"] = msg.User.Name
	}

	// Badges (for events like raids, we may want these)
	if len(msg.User.Badges) > 0 {
		badges := make([]string, 0, len(msg.User.Badges))
		for badge, version := range msg.User.Badges {
			badges = append(badges, fmt.Sprintf("%s/%d", badge, version))
		}
		tags["badges"] = strings.Join(badges, ",")
	}

	// Room ID
	if msg.RoomID != "" {
		tags["room-id"] = msg.RoomID
	}

	// Create RawChatMessage with event fields
	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: strings.ToLower(strings.TrimPrefix(msg.Channel, "#")),
		UserID:    msg.User.ID,
		Username:  strings.ToLower(msg.User.Name),
		Text:      msg.SystemMsg, // Use system message as display text
		Timestamp: time.Now().UTC(),
		Tags:      tags,
		EventType: eventType,
		EventData: eventData,
	}

	// Use message timestamp if available
	if msg.Time.Unix() > 0 {
		rawMsg.Timestamp = msg.Time.UTC()
	}

	return rawMsg, nil
}

// isCoveredByEventSub reports whether the given USERNOTICE msg-id corresponds
// to an event type the twitch-eventsub-listener already subscribes to. The
// EventSub variants carry strictly richer data (cumulative_months, gift counts
// from channel.subscription.gift, viewer counts from channel.raid) so emitting
// them from IRC too is pure duplication.
func isCoveredByEventSub(msgID string) bool {
	switch msgID {
	case "sub", "resub", "subgift", "anonsubgift", "submysterygift", "raid":
		return true
	default:
		return false
	}
}

// mapMsgIDToEventType converts Twitch msg-id to our event type taxonomy
func mapMsgIDToEventType(msgID string) string {
	switch msgID {
	case "sub":
		return "subscription"
	case "resub":
		return "resubscription"
	case "subgift":
		return "gift_subscription"
	case "anonsubgift":
		return "gift_subscription"
	case "submysterygift":
		return "mystery_gift"
	case "giftpaidupgrade":
		return "gift_paid_upgrade"
	case "rewardgift":
		return "reward_gift"
	case "anongiftpaidupgrade":
		return "anon_gift_paid_upgrade"
	case "raid":
		return "raid"
	case "unraid":
		return "unraid"
	case "ritual":
		return "ritual"
	case "bitsbadgetier":
		return "bits"
	default:
		return "unknown"
	}
}

// extractEventDataFromTags extracts event-specific data from msg-param-* tags
func extractEventDataFromTags(tags map[string]string, msgID string) map[string]interface{} {
	data := make(map[string]interface{})

	switch msgID {
	case "sub", "resub":
		// Subscription and resubscription
		// msg-param-sub-plan: "1000", "2000", "3000", "Prime"
		// msg-param-cumulative-months: "12"
		// msg-param-streak-months: "6"
		// msg-param-sub-plan-name: "Channel Subscription (channelname)"
		if plan, ok := tags["msg-param-sub-plan"]; ok {
			data["tier"] = plan
		}
		if months, ok := tags["msg-param-cumulative-months"]; ok {
			if val, err := strconv.Atoi(months); err == nil {
				data["months"] = val
			}
		}
		if streak, ok := tags["msg-param-streak-months"]; ok {
			if val, err := strconv.Atoi(streak); err == nil {
				data["streak_months"] = val
			}
		}
		if planName, ok := tags["msg-param-sub-plan-name"]; ok {
			data["plan_name"] = planName
		}
		data["is_gift"] = false

	case "subgift", "anonsubgift":
		// Gift subscriptions
		// msg-param-recipient-id, msg-param-recipient-user-name
		// msg-param-sub-plan, msg-param-months
		// msg-param-sender-count (total gifts by sender)
		if plan, ok := tags["msg-param-sub-plan"]; ok {
			data["tier"] = plan
		}
		if recipientID, ok := tags["msg-param-recipient-id"]; ok {
			data["recipient_id"] = recipientID
		}
		if recipientName, ok := tags["msg-param-recipient-user-name"]; ok {
			data["recipient_name"] = recipientName
		}
		if months, ok := tags["msg-param-months"]; ok {
			if val, err := strconv.Atoi(months); err == nil {
				data["months"] = val
			}
		}
		if senderCount, ok := tags["msg-param-sender-count"]; ok {
			if val, err := strconv.Atoi(senderCount); err == nil {
				data["sender_total_gifts"] = val
			}
		}
		data["is_gift"] = true
		data["is_anonymous"] = msgID == "anonsubgift"

	case "submysterygift":
		// Mystery gift bombs
		// msg-param-mass-gift-count, msg-param-sub-plan
		// msg-param-sender-count (total gifts by sender)
		if plan, ok := tags["msg-param-sub-plan"]; ok {
			data["tier"] = plan
		}
		if count, ok := tags["msg-param-mass-gift-count"]; ok {
			if val, err := strconv.Atoi(count); err == nil {
				data["gift_count"] = val
			}
		}
		if senderCount, ok := tags["msg-param-sender-count"]; ok {
			if val, err := strconv.Atoi(senderCount); err == nil {
				data["sender_total_gifts"] = val
			}
		}

	case "raid", "unraid":
		// Raid events
		// msg-param-displayName, msg-param-login, msg-param-viewerCount
		if displayName, ok := tags["msg-param-displayName"]; ok {
			data["raider_channel"] = displayName
		}
		if login, ok := tags["msg-param-login"]; ok {
			data["raider_login"] = login
		}
		if viewers, ok := tags["msg-param-viewerCount"]; ok {
			if val, err := strconv.Atoi(viewers); err == nil {
				data["viewer_count"] = val
			}
		}
		if systemMsg, ok := tags["system-msg"]; ok {
			data["system_message"] = systemMsg
		}

	case "bitsbadgetier":
		// Bits badge tier achievement
		// msg-param-threshold: "1000", "5000", etc.
		if threshold, ok := tags["msg-param-threshold"]; ok {
			if val, err := strconv.Atoi(threshold); err == nil {
				data["badge_tier"] = val
			}
		}

	case "ritual":
		// First-time chatter ritual
		// msg-param-ritual-name: "new_chatter"
		if ritualName, ok := tags["msg-param-ritual-name"]; ok {
			data["ritual_name"] = ritualName
		}
	}

	return data
}
