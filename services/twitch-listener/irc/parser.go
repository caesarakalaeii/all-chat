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

// Parser handles parsing Twitch IRC messages into RawChatMessage format
type Parser struct{}

// NewParser creates a new IRC message parser
func NewParser() *Parser {
	return &Parser{}
}

// ParsePrivateMessage converts a Twitch IRC PRIVMSG into a RawChatMessage
func (p *Parser) ParsePrivateMessage(msg twitch.PrivateMessage) (*models.RawChatMessage, error) {
	if msg.Channel == "" {
		return nil, fmt.Errorf("missing channel")
	}

	if msg.User.Name == "" {
		return nil, fmt.Errorf("missing username")
	}

	if msg.Message == "" {
		return nil, fmt.Errorf("missing message text")
	}

	// Extract tags from Twitch IRC message
	tags := make(map[string]string)

	// User info
	if msg.User.ID != "" {
		tags["user-id"] = msg.User.ID
	}
	if msg.User.DisplayName != "" {
		tags["display-name"] = msg.User.DisplayName
	}
	if msg.User.Color != "" {
		tags["color"] = msg.User.Color
	}

	// Badges
	if len(msg.User.Badges) > 0 {
		badges := make([]string, 0, len(msg.User.Badges))
		for badge, version := range msg.User.Badges {
			badges = append(badges, fmt.Sprintf("%s/%d", badge, version))
		}
		tags["badges"] = strings.Join(badges, ",")
	}

	// User type flags (extract from badges instead)
	tags["subscriber"] = "0"
	tags["mod"] = "0"
	tags["turbo"] = "0"

	// Check badges for subscriber, moderator, turbo status
	for badge := range msg.User.Badges {
		switch badge {
		case "subscriber":
			tags["subscriber"] = "1"
		case "moderator":
			tags["mod"] = "1"
		case "turbo":
			tags["turbo"] = "1"
		}
	}

	// Emotes (Twitch native emotes with positions)
	if len(msg.Emotes) > 0 {
		emoteStrs := make([]string, 0, len(msg.Emotes))
		for _, emote := range msg.Emotes {
			// Format: emoteID:startPos-endPos
			positions := make([]string, 0, len(emote.Positions))
			for _, pos := range emote.Positions {
				positions = append(positions, fmt.Sprintf("%d-%d", pos.Start, pos.End))
			}
			emoteStrs = append(emoteStrs, fmt.Sprintf("%s:%s", emote.ID, strings.Join(positions, ",")))
		}
		tags["emotes"] = strings.Join(emoteStrs, "/")
	}

	// Shared Chat tags (for multi-channel collaborative streams)
	// These tags indicate the message's true origin when channels share chat
	if msg.Tags["source-room-id"] != "" {
		tags["source-room-id"] = msg.Tags["source-room-id"]
	}

	if msg.Tags["source-id"] != "" {
		tags["source-id"] = msg.Tags["source-id"]
	}

	if msg.Tags["source-badges"] != "" {
		tags["source-badges"] = msg.Tags["source-badges"]
	}

	if msg.Tags["source-badge-info"] != "" {
		tags["source-badge-info"] = msg.Tags["source-badge-info"]
	}

	// Chat GIFs (ADR-0037): forward Twitch's native "gifs" tag ("start-end|gif_id|url,...") so the
	// message-processor renders IRC-sourced chat GIFs identically to EventSub-sourced ones. Unlike
	// the standard tags above, gempir does not surface this on a typed field, so it is copied
	// verbatim from the raw tag map.
	if msg.Tags["gifs"] != "" {
		tags["gifs"] = msg.Tags["gifs"]
	}

	// Message metadata
	tags["id"] = msg.ID
	if msg.RoomID != "" {
		tags["room-id"] = msg.RoomID
	}
	if msg.Time.Unix() > 0 {
		tags["tmi-sent-ts"] = fmt.Sprintf("%d", msg.Time.UnixMilli())
	}

	// Extract bits (for cheermote messages)
	if msg.Bits > 0 {
		tags["bits"] = strconv.Itoa(msg.Bits)
	}

	// Create RawChatMessage
	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: strings.ToLower(strings.TrimPrefix(msg.Channel, "#")),
		UserID:    msg.User.ID,
		Username:  strings.ToLower(msg.User.Name),
		Text:      msg.Message,
		Timestamp: time.Now().UTC(),
		Tags:      tags,
	}

	// Use Twitch timestamp if available
	if msg.Time.Unix() > 0 {
		rawMsg.Timestamp = msg.Time.UTC()
	}

	return rawMsg, nil
}

// ParseClearMessage converts Twitch CLEARMSG to RawChatMessage deletion event
func (p *Parser) ParseClearMessage(msg twitch.ClearMessage) *models.RawChatMessage {
	channelName := strings.TrimPrefix(msg.Channel, "#")

	return &models.RawChatMessage{
		MessageID: uuid.New().String(), // New UUID for deletion event itself
		Platform:  "twitch",
		ChannelID: channelName,
		UserID:    "",        // Not provided in CLEARMSG
		Username:  msg.Login, // User whose message was deleted (if available)
		Text:      "",        // No message text for deletion events
		Timestamp: time.Now().UTC(),
		Tags:      msg.Tags, // Preserve IRC tags for debugging
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": msg.TargetMsgID, // Platform message ID to delete
		},
	}
}

// ParseClearChat converts Twitch CLEARCHAT to RawChatMessage deletion event
func (p *Parser) ParseClearChat(msg twitch.ClearChatMessage) *models.RawChatMessage {
	channelName := strings.TrimPrefix(msg.Channel, "#")

	// Determine deletion type: batch (user timeout/ban) or clear (full chat)
	deletionType := "clear"
	eventData := map[string]interface{}{
		"deletion_type": deletionType,
	}

	if msg.TargetUserID != "" {
		// User timeout or ban - batch deletion
		deletionType = "batch"
		eventData["deletion_type"] = deletionType
		eventData["target_user_id"] = msg.TargetUserID
		eventData["target_username"] = msg.TargetUsername

		if msg.BanDuration > 0 {
			eventData["ban_duration"] = msg.BanDuration // Timeout duration in seconds
		}
	}

	return &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: channelName,
		UserID:    msg.TargetUserID,   // Empty for full clear
		Username:  msg.TargetUsername, // Empty for full clear
		Text:      "",
		Timestamp: time.Now().UTC(),
		Tags:      msg.Tags,
		EventType: "message_deletion",
		EventData: eventData,
	}
}

// boolToString converts a boolean to "1" or "0"
func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
