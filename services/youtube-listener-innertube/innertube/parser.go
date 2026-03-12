package innertube

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/deletion"
	"github.com/google/uuid"
)

// Package-level batch detector for deletion event processing
var (
	batchDetector   *deletion.BatchDetector
	batchDetectorMu sync.RWMutex
)

// SetBatchDetector configures the batch detector for deletion event processing
// This should be called once during service initialization
func SetBatchDetector(detector *deletion.BatchDetector) {
	batchDetectorMu.Lock()
	defer batchDetectorMu.Unlock()
	batchDetector = detector
}

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

// GetMessageID returns the message ID (implements deletion.RawMessage interface)
func (m *RawChatMessage) GetMessageID() string {
	return m.MessageID
}

// GetChannelID returns the channel ID (implements deletion.RawMessage interface)
func (m *RawChatMessage) GetChannelID() string {
	return m.ChannelID
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

		// Handle ticker items specially (they contain pinned Super Chats/Stickers)
		if action.AddLiveChatTickerItem != nil {
			tickerMsg, err := parseTickerEvent(action.AddLiveChatTickerItem, channelID)
			if err != nil {
				// Log error but continue processing
				continue
			}
			if tickerMsg != nil {
				messages = append(messages, tickerMsg)
			}
			continue
		}

		// Handle deletion events
		if action.MarkChatItemAsDeletedAction != nil {
			delEvent, err := parseDeletionEvent(action.MarkChatItemAsDeletedAction, channelID)
			if err != nil {
				// Log error but continue processing other messages
				continue
			}
			messages = append(messages, delEvent)
			continue
		}

		// Extract the actual chat item for regular messages
		var item *ChatItem
		if action.AddChatItemAction != nil {
			item = &action.AddChatItemAction.Item
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

	// Strip @ prefix from username if present (YouTube returns @username)
	username := renderer.AuthorName.SimpleText
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "", // Not provided by InnerTube - set by control plane in Phase 10
		UserID:    renderer.AuthorExternalChannelID,
		Username:  username,
		Text:      text,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
	}

	// Add avatar URL from AuthorPhoto thumbnails
	if avatarURL := bestThumbnailURL(renderer.AuthorPhoto); avatarURL != "" {
		msg.Tags["profile_image"] = avatarURL
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

	// Build rich event data
	eventData := map[string]interface{}{
		"amount": renderer.PurchaseAmountText.SimpleText,
	}

	// Add amount in micros if available (for sorting by amount)
	if renderer.AmountMicros > 0 {
		eventData["amount_micros"] = renderer.AmountMicros
	}

	// Add color tier if available (for overlay styling)
	if renderer.HeaderBackgroundColor != 0 {
		eventData["color"] = formatColorFromInt(renderer.HeaderBackgroundColor)
	}

	// Strip @ prefix from username if present (YouTube returns @username)
	username := renderer.AuthorName.SimpleText
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "",
		UserID:    renderer.AuthorExternalChannelID,
		Username:  username,
		Text:      text,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		EventType: "super_chat",
		EventData: eventData,
	}

	if avatarURL := bestThumbnailURL(renderer.AuthorPhoto); avatarURL != "" {
		msg.Tags["profile_image"] = avatarURL
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

	// Distinguish between welcome and milestone based on text content
	eventType := "member_joined"
	eventData := map[string]interface{}{
		"level_name": "Member", // Default level name
	}

	// Check if this is a milestone (e.g., "Member for 6 months")
	months := extractMilestoneMonths(text)
	if months > 0 {
		eventType = "member_milestone"
		eventData["months"] = months
	}

	// Extract level name from badges if available
	if len(renderer.AuthorBadges) > 0 {
		badges := extractBadges(renderer.AuthorBadges)
		if len(badges) > 0 {
			// Use first badge as level name (typically the membership badge)
			eventData["level_name"] = badges[0]
		}
	}

	// Strip @ prefix from username if present (YouTube returns @username)
	username := renderer.AuthorName.SimpleText
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "",
		UserID:    renderer.AuthorExternalChannelID,
		Username:  username,
		Text:      text,
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		EventType: eventType,
		EventData: eventData,
	}

	if avatarURL := bestThumbnailURL(renderer.AuthorPhoto); avatarURL != "" {
		msg.Tags["profile_image"] = avatarURL
	}

	return msg, nil
}

// parsePaidSticker converts a LiveChatPaidStickerRenderer to RawChatMessage
func parsePaidSticker(renderer *LiveChatPaidStickerRenderer, channelID string) (*RawChatMessage, error) {
	timestamp, err := parseTimestampUsec(renderer.TimestampUsec)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	// Build rich event data
	eventData := map[string]interface{}{
		"amount": renderer.PurchaseAmountText.SimpleText,
	}

	// Add amount in micros if available
	if renderer.AmountMicros > 0 {
		eventData["amount_micros"] = renderer.AmountMicros
	}

	// Extract sticker URL from thumbnails
	if len(renderer.Sticker.Thumbnails.Thumbnails) > 0 {
		eventData["sticker_url"] = renderer.Sticker.Thumbnails.Thumbnails[0].URL
	}

	// Strip @ prefix from username if present (YouTube returns @username)
	username := renderer.AuthorName.SimpleText
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}

	msg := &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "",
		UserID:    renderer.AuthorExternalChannelID,
		Username:  username,
		Text:      "", // Stickers have no text content (changed from "[sticker]" for consistency)
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		EventType: "super_sticker",
		EventData: eventData,
	}

	if avatarURL := bestThumbnailURL(renderer.AuthorPhoto); avatarURL != "" {
		msg.Tags["profile_image"] = avatarURL
	}

	return msg, nil
}

// parseTickerEvent converts a AddLiveChatTickerItem (pinned events) to RawChatMessage
func parseTickerEvent(ticker *AddLiveChatTickerItem, channelID string) (*RawChatMessage, error) {
	// Ticker wraps an underlying event (Super Chat or Super Sticker)
	// Parse the underlying event first
	var msg *RawChatMessage
	var err error

	item := &ticker.Item
	if item.LiveChatPaidMessageRenderer != nil {
		msg, err = parsePaidMessage(item.LiveChatPaidMessageRenderer, channelID)
	} else if item.LiveChatPaidStickerRenderer != nil {
		msg, err = parsePaidSticker(item.LiveChatPaidStickerRenderer, channelID)
	} else {
		// Ticker contains unsupported event type - skip
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("parse ticker underlying event: %w", err)
	}

	// Add ticker metadata to the event data
	if msg.EventData == nil {
		msg.EventData = make(map[string]interface{})
	}
	msg.EventData["pinned"] = true

	if ticker.DurationSec > 0 {
		msg.EventData["ticker_duration_sec"] = ticker.DurationSec
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

// formatColorFromInt converts an integer color value to hex string
func formatColorFromInt(color int) string {
	return fmt.Sprintf("#%06X", color&0xFFFFFF)
}

// extractMilestoneMonths extracts the month count from milestone text
// Example: "Member for 6 months" -> 6
func extractMilestoneMonths(text string) int {
	// Look for pattern: "Member for N month(s)" or similar variations
	text = strings.ToLower(text)

	// Try to find "N month" or "N months" patterns
	if strings.Contains(text, "month") {
		// Extract just the numeric part before "month"
		parts := strings.Split(text, "month")
		if len(parts) > 0 {
			// Look for the last number before "month"
			words := strings.Fields(parts[0])
			for i := len(words) - 1; i >= 0; i-- {
				if months, err := strconv.Atoi(words[i]); err == nil && months > 0 {
					return months
				}
			}
		}
	}

	return 0
}

// bestThumbnailURL returns the URL of the largest thumbnail from a Thumbnails list,
// or the last one if dimensions are not available. Returns "" if the list is empty.
func bestThumbnailURL(t Thumbnails) string {
	if len(t.Thumbnails) == 0 {
		return ""
	}
	best := t.Thumbnails[0]
	for _, thumb := range t.Thumbnails[1:] {
		if thumb.Width*thumb.Height > best.Width*best.Height {
			best = thumb
		}
	}
	return best.URL
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

// ToJSON converts the RawChatMessage to JSON bytes
// Used for publishing the full message payload to Redis Streams 'data' field
func (msg *RawChatMessage) ToJSON() ([]byte, error) {
	return json.Marshal(msg)
}

// parseDeletionEvent converts a MarkChatItemAsDeletedAction to RawChatMessage
// Uses package-level batch detector if configured for batch deletion detection
func parseDeletionEvent(action *MarkChatItemAsDeletedAction, channelID string) (*RawChatMessage, error) {
	// Extract deletion timestamp (use current time if not provided)
	timestamp := time.Now().UTC()
	if action.TimestampUsec != "" {
		if ts, err := parseTimestampUsec(action.TimestampUsec); err == nil {
			timestamp = ts
		}
	}

	// Extract deleted message ID from TargetItemID
	// This is the InnerTube internal ID (not the YouTube message ID)
	deletedMessageID := action.TargetItemID
	if deletedMessageID == "" {
		return nil, fmt.Errorf("deletion event missing target item ID")
	}

	// Default deletion metadata (single deletion)
	deletionType := "single"
	var deletionCount *int
	var reason *string

	// If batch detector configured, add deletion for batch detection
	batchDetectorMu.RLock()
	detector := batchDetector
	batchDetectorMu.RUnlock()

	if detector != nil {
		// Add deletion to detector for batch analysis
		// AddDeletion returns BatchResult immediately when threshold crossed
		batchResult, err := detector.AddDeletion(channelID, deletedMessageID, timestamp)
		if err != nil {
			// Log error but continue with single deletion event
			// Batch detection is optional, don't fail message processing
		}

		// Check if this deletion triggered batch detection
		if batchResult != nil && batchResult.IsBatch {
			deletionType = "batch"
			deletionCount = &batchResult.Count
			reason = &batchResult.Reason
		}
	}

	// Create deletion event message matching official listener schema
	msg := &RawChatMessage{
		MessageID: uuid.New().String(), // New ID for deletion event itself
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  "", // Populated by caller
		UserID:    "", // Deletion events don't have a user (moderator action)
		Username:  "", // No username for deletion events
		Text:      "", // Deletion events have no text
		Timestamp: timestamp,
		Tags:      make(map[string]string),
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"target_msg_id":  deletedMessageID, // Match official listener schema
			"deletion_type":  deletionType,
		},
	}

	// Add batch metadata if applicable
	if deletionCount != nil {
		msg.EventData["deletion_count"] = *deletionCount
	}
	if reason != nil {
		msg.EventData["reason"] = *reason
	}

	return msg, nil
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

	// Deletion events don't have UserID/Username - skip validation for them
	if msg.EventType != "message_deletion" {
		if msg.UserID == "" {
			return fmt.Errorf("UserID is required")
		}
		if msg.Username == "" {
			return fmt.Errorf("Username is required")
		}
		if msg.Text == "" && msg.EventType == "" {
			return fmt.Errorf("Text is required for non-event messages")
		}
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
