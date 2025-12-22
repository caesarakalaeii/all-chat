package irc

import (
	"fmt"
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

	// Message metadata
	tags["id"] = msg.ID
	if msg.RoomID != "" {
		tags["room-id"] = msg.RoomID
	}
	if msg.Time.Unix() > 0 {
		tags["tmi-sent-ts"] = fmt.Sprintf("%d", msg.Time.UnixMilli())
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

// boolToString converts a boolean to "1" or "0"
func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
