package innertube

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RawChatMessage represents a raw chat message matching the official youtube-listener schema
// This must remain byte-for-byte compatible with services/youtube-listener/models/raw_message.go
type RawChatMessage struct {
	MessageID string            `json:"message_id"` // UUID
	Platform  string            `json:"platform"`   // "youtube"
	ChannelID string            `json:"channel_id"` // YouTube channel ID
	StreamID  string            `json:"stream_id"`  // YouTube live stream ID (optional for PoC)
	UserID    string            `json:"user_id"`    // YouTube user channel ID
	Username  string            `json:"username"`   // Display name
	Text      string            `json:"text"`       // Message text
	Timestamp time.Time         `json:"timestamp"`  // UTC timestamp
	Tags      map[string]string `json:"tags"`       // YouTube-specific metadata

	// Event support (omitted for regular chat messages - deferred to Phase 13)
	EventType string                 `json:"event_type,omitempty"` // "super_chat", "member_milestone", etc.
	EventData map[string]interface{} `json:"event_data,omitempty"` // Event-specific payload
}

// ParseMessages converts InnerTube ChatAction objects to RawChatMessage format
// Returns only valid messages - invalid messages are logged and skipped
func ParseMessages(actions []ChatAction, channelID string) ([]*RawChatMessage, error) {
	var messages []*RawChatMessage

	for i, action := range actions {
		// Handle nested replay actions
		if action.ReplayChatItemAction != nil {
			replayMessages, err := ParseMessages(action.ReplayChatItemAction.Actions, channelID)
			if err != nil {
				return nil, fmt.Errorf("parse replay action %d: %w", i, err)
			}
			messages = append(messages, replayMessages...)
			continue
		}

		// Extract the actual chat item
		var item *ChatItem
		if action.AddChatItemAction != nil {
			item = &action.AddChatItemAction.Item
		} else if action.AddLiveChatTickerItem != nil {
			item = &action.AddLiveChatTickerItem.Item
		}

		if item == nil {
			// Skip actions without chat items (e.g., banners, tooltips)
			continue
		}

		// Parse based on message type
		var msg *RawChatMessage
		var err error

		if item.LiveChatTextMessageRenderer != nil {
			msg, err = parseTextMessage(item.LiveChatTextMessageRenderer, channelID)
		} else if item.LiveChatPaidMessageRenderer != nil {
			msg, err = parsePaidMessage(item.LiveChatPaidMessageRenderer, channelID)
		} else if item.LiveChatMembershipItemRenderer != nil {
			msg, err = parseMembershipMessage(item.LiveChatMembershipItemRenderer, channelID)
		} else if item.LiveChatPaidStickerRenderer != nil {
			msg, err = parsePaidSticker(item.LiveChatPaidStickerRenderer, channelID)
		} else {
			// Unknown message type - skip
			continue
		}

		if err != nil {
			// Log error but continue processing other messages
			// In production, this would use the logger passed from the service
			continue
		}

		// Validate before adding to results
		if err := ValidateRawMessage(msg); err != nil {
			// Skip invalid messages
			continue
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// parseTextMessage converts a LiveChatTextMessageRenderer to RawChatMessage
func parseTextMessage(renderer *LiveChatTextMessageRenderer, channelID string) (*RawChatMessage, error) {
	timestamp, err := parseTimestampUsec(renderer.TimestampUsec)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	text := extractMessageText(renderer.Message)

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "", // Not provided by InnerTube - set by control plane in Phase 10
		UserID:    renderer.AuthorExternalChannelID,
		Username:  renderer.AuthorName.SimpleText,
		Text:      text,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
	}

	// Add badges to tags if present
	if len(renderer.AuthorBadges) > 0 {
		badges := extractBadges(renderer.AuthorBadges)
		if len(badges) > 0 {
			msg.Tags["badges"] = strings.Join(badges, ",")
		}
	}

	return msg, nil
}

// parsePaidMessage converts a LiveChatPaidMessageRenderer (Super Chat) to RawChatMessage
func parsePaidMessage(renderer *LiveChatPaidMessageRenderer, channelID string) (*RawChatMessage, error) {
	timestamp, err := parseTimestampUsec(renderer.TimestampUsec)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	// Message may be empty for some Super Chats
	text := ""
	if renderer.Message.Runs != nil {
		text = extractMessageText(renderer.Message)
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "",
		UserID:    renderer.AuthorExternalChannelID,
		Username:  renderer.AuthorName.SimpleText,
		Text:      text,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		// Event data deferred to Phase 13
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount": renderer.PurchaseAmountText.SimpleText,
		},
	}

	return msg, nil
}

// parseMembershipMessage converts a LiveChatMembershipItemRenderer to RawChatMessage
func parseMembershipMessage(renderer *LiveChatMembershipItemRenderer, channelID string) (*RawChatMessage, error) {
	timestamp, err := parseTimestampUsec(renderer.TimestampUsec)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	// Extract message from header subtext
	text := ""
	if renderer.HeaderSubtext.Runs != nil {
		text = extractMessageText(renderer.HeaderSubtext)
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "",
		UserID:    renderer.AuthorExternalChannelID,
		Username:  renderer.AuthorName.SimpleText,
		Text:      text,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		// Event data deferred to Phase 13
		EventType: "membership",
		EventData: make(map[string]interface{}),
	}

	return msg, nil
}

// parsePaidSticker converts a LiveChatPaidStickerRenderer to RawChatMessage
func parsePaidSticker(renderer *LiveChatPaidStickerRenderer, channelID string) (*RawChatMessage, error) {
	timestamp, err := parseTimestampUsec(renderer.TimestampUsec)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "",
		UserID:    renderer.AuthorExternalChannelID,
		Username:  renderer.AuthorName.SimpleText,
		Text:      "[sticker]", // Stickers have no text content
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		// Event data deferred to Phase 13
		EventType: "paid_sticker",
		EventData: map[string]interface{}{
			"amount": renderer.PurchaseAmountText.SimpleText,
		},
	}

	return msg, nil
}

// extractMessageText concatenates all text runs into a single string
func extractMessageText(message MessageContent) string {
	var parts []string
	for _, run := range message.Runs {
		if run.Text != "" {
			parts = append(parts, run.Text)
		} else if run.Emoji != nil && len(run.Emoji.Shortcuts) > 0 {
			// Use first shortcut as emoji text representation
			parts = append(parts, run.Emoji.Shortcuts[0])
		}
	}
	return strings.Join(parts, "")
}

// extractBadges extracts badge types from author badges
func extractBadges(badges []AuthorBadge) []string {
	var badgeTypes []string
	for _, badge := range badges {
		if badge.LiveChatAuthorBadgeRenderer.Tooltip != "" {
			badgeTypes = append(badgeTypes, badge.LiveChatAuthorBadgeRenderer.Tooltip)
		}
	}
	return badgeTypes
}

// parseTimestampUsec converts a timestampUsec string to time.Time
func parseTimestampUsec(timestampUsec string) (time.Time, error) {
	if timestampUsec == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	usec, err := strconv.ParseInt(timestampUsec, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp integer: %w", err)
	}

	// Convert microseconds to seconds and nanoseconds
	sec := usec / 1000000
	nsec := (usec % 1000000) * 1000

	return time.Unix(sec, nsec).UTC(), nil
}

// ValidateRawMessage ensures the message conforms to the official youtube-listener schema
// Critical fields must be non-empty, optional fields get sensible defaults
func ValidateRawMessage(msg *RawChatMessage) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	// Critical fields - must be non-empty
	if msg.MessageID == "" {
		return fmt.Errorf("MessageID is required")
	}
	if msg.Platform == "" {
		return fmt.Errorf("Platform is required")
	}
	if msg.Platform != "youtube" {
		return fmt.Errorf("Platform must be 'youtube', got '%s'", msg.Platform)
	}
	if msg.UserID == "" {
		return fmt.Errorf("UserID is required")
	}
	if msg.Username == "" {
		return fmt.Errorf("Username is required")
	}
	if msg.Text == "" && msg.EventType == "" {
		return fmt.Errorf("Text is required for non-event messages")
	}
	if msg.Timestamp.IsZero() {
		return fmt.Errorf("Timestamp is required")
	}

	// Optional fields - apply sensible defaults
	if msg.Tags == nil {
		msg.Tags = make(map[string]string)
	}
	if msg.ChannelID == "" {
		// ChannelID is technically optional but should be set by the caller
		// Don't fail validation, just ensure it's not nil
	}

	return nil
}
