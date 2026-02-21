package innertube

import (
	"errors"
	"fmt"
	"time"
)

// LiveChatResponse represents the InnerTube API response structure
// Minimal implementation for PoC - only fields needed for message parsing
type LiveChatResponse struct {
	ContinuationContents ContinuationContents `json:"continuationContents"`
}

// ContinuationContents contains the live chat continuation data
type ContinuationContents struct {
	LiveChatContinuation LiveChatContinuation `json:"liveChatContinuation"`
}

// LiveChatContinuation contains actions and the next continuation token
type LiveChatContinuation struct {
	Actions       []ChatAction   `json:"actions,omitempty"`
	Continuations []Continuation `json:"continuations,omitempty"`
}

// ChatAction wraps different types of chat actions
type ChatAction struct {
	AddChatItemAction      *AddChatItemAction      `json:"addChatItemAction,omitempty"`
	AddLiveChatTickerItem  *AddLiveChatTickerItem  `json:"addLiveChatTickerItemAction,omitempty"`
	ReplayChatItemAction   *ReplayChatItemAction   `json:"replayChatItemAction,omitempty"`
}

// AddChatItemAction represents a new chat message being added
type AddChatItemAction struct {
	Item ChatItem `json:"item"`
}

// ReplayChatItemAction represents a chat message in replay mode
type ReplayChatItemAction struct {
	Actions []ChatAction `json:"actions"`
}

// AddLiveChatTickerItem represents ticker items (super chats, memberships)
type AddLiveChatTickerItem struct {
	Item        ChatItem `json:"item"`
	DurationSec int      `json:"durationSec,omitempty"` // Ticker display duration
}

// ChatItem can contain different message types
type ChatItem struct {
	LiveChatTextMessageRenderer           *LiveChatTextMessageRenderer           `json:"liveChatTextMessageRenderer,omitempty"`
	LiveChatPaidMessageRenderer           *LiveChatPaidMessageRenderer           `json:"liveChatPaidMessageRenderer,omitempty"`
	LiveChatMembershipItemRenderer        *LiveChatMembershipItemRenderer        `json:"liveChatMembershipItemRenderer,omitempty"`
	LiveChatPaidStickerRenderer           *LiveChatPaidStickerRenderer           `json:"liveChatPaidStickerRenderer,omitempty"`
}

// LiveChatTextMessageRenderer represents a standard text chat message
type LiveChatTextMessageRenderer struct {
	Message              MessageContent `json:"message"`
	AuthorName           SimpleText     `json:"authorName"`
	AuthorExternalChannelID string      `json:"authorExternalChannelId"`
	TimestampUsec        string         `json:"timestampUsec"` // Microseconds as string
	AuthorPhoto          Thumbnails     `json:"authorPhoto,omitempty"`
	AuthorBadges         []AuthorBadge  `json:"authorBadges,omitempty"`
}

// LiveChatPaidMessageRenderer represents a Super Chat message
type LiveChatPaidMessageRenderer struct {
	Message              MessageContent `json:"message,omitempty"`
	AuthorName           SimpleText     `json:"authorName"`
	AuthorExternalChannelID string      `json:"authorExternalChannelId"`
	TimestampUsec        string         `json:"timestampUsec"`
	AuthorPhoto          Thumbnails     `json:"authorPhoto,omitempty"`
	PurchaseAmountText   SimpleText     `json:"purchaseAmountText"`
	AmountMicros         int64          `json:"purchaseAmountMicros,omitempty"` // For sorting by amount
	HeaderBackgroundColor int           `json:"headerBackgroundColor,omitempty"` // Color tier indicator
}

// LiveChatMembershipItemRenderer represents a membership/join message
type LiveChatMembershipItemRenderer struct {
	HeaderSubtext        MessageContent `json:"headerSubtext,omitempty"`
	AuthorName           SimpleText     `json:"authorName"`
	AuthorExternalChannelID string      `json:"authorExternalChannelId"`
	TimestampUsec        string         `json:"timestampUsec"`
	AuthorPhoto          Thumbnails     `json:"authorPhoto,omitempty"`
	AuthorBadges         []AuthorBadge  `json:"authorBadges,omitempty"`
}

// LiveChatPaidStickerRenderer represents a Super Sticker
type LiveChatPaidStickerRenderer struct {
	AuthorName           SimpleText     `json:"authorName"`
	AuthorExternalChannelID string      `json:"authorExternalChannelId"`
	TimestampUsec        string         `json:"timestampUsec"`
	AuthorPhoto          Thumbnails     `json:"authorPhoto,omitempty"`
	PurchaseAmountText   SimpleText     `json:"purchaseAmountText"`
	Sticker              StickerContent `json:"sticker,omitempty"`
	AmountMicros         int64          `json:"purchaseAmountMicros,omitempty"` // For sorting by amount
}

// MessageContent represents the message text with runs (segments)
type MessageContent struct {
	Runs []MessageRun `json:"runs,omitempty"`
}

// MessageRun represents a segment of the message (text or emoji)
type MessageRun struct {
	Text  string      `json:"text,omitempty"`
	Emoji *EmojiData  `json:"emoji,omitempty"`
}

// EmojiData represents emoji metadata
type EmojiData struct {
	EmojiID   string     `json:"emojiId,omitempty"`
	Shortcuts []string   `json:"shortcuts,omitempty"`
	Image     Thumbnails `json:"image,omitempty"`
}

// SimpleText represents a simple text field
type SimpleText struct {
	SimpleText string `json:"simpleText"`
}

// Thumbnails represents a list of thumbnail images
type Thumbnails struct {
	Thumbnails []Thumbnail `json:"thumbnails,omitempty"`
}

// Thumbnail represents a single thumbnail with URL and dimensions
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// AuthorBadge represents a user badge (moderator, member, verified, etc.)
type AuthorBadge struct {
	LiveChatAuthorBadgeRenderer LiveChatAuthorBadgeRenderer `json:"liveChatAuthorBadgeRenderer"`
}

// LiveChatAuthorBadgeRenderer contains badge details
type LiveChatAuthorBadgeRenderer struct {
	CustomThumbnail Thumbnails `json:"customThumbnail,omitempty"`
	Icon            *IconData  `json:"icon,omitempty"`
	Tooltip         string     `json:"tooltip,omitempty"`
}

// IconData represents icon metadata
type IconData struct {
	IconType string `json:"iconType"`
}

// StickerContent represents sticker data
type StickerContent struct {
	Thumbnails Thumbnails `json:"thumbnails,omitempty"`
}

// Continuation contains the continuation token for pagination
type Continuation struct {
	TimedContinuationData       *TimedContinuationData       `json:"timedContinuationData,omitempty"`
	InvalidationContinuationData *InvalidationContinuationData `json:"invalidationContinuationData,omitempty"`
	LiveChatReplayContinuationData *LiveChatReplayContinuationData `json:"liveChatReplayContinuationData,omitempty"`
}

// TimedContinuationData contains continuation token with timeout
type TimedContinuationData struct {
	TimeoutDurationMillis int64  `json:"timeoutDurationMillis"`
	Continuation          string `json:"continuation"`
}

// InvalidationContinuationData contains continuation for invalidation
type InvalidationContinuationData struct {
	TimeoutDurationMillis int64  `json:"timeoutDurationMillis"`
	Continuation          string `json:"continuation"`
}

// LiveChatReplayContinuationData contains continuation for replay
type LiveChatReplayContinuationData struct {
	TimeUntilLastMessageMsec int64  `json:"timeUntilLastMessageMsec"`
	Continuation             string `json:"continuation"`
}

// HTTPStatusError represents an HTTP error response
type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return "HTTP " + string(rune(e.StatusCode)) + ": " + e.Body
}

// ErrorType classifies error types for handling strategy
type ErrorType int

const (
	ErrorTypeTransient ErrorType = iota // Retry with backoff
	ErrorTypeFatal                       // Stop monitoring
)

// ClassifyError determines if an error is transient or fatal
func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeTransient
	}

	// HTTP status errors (unwrap error chain if wrapped)
	var httpErr *HTTPStatusError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 401, 403, 404:
			return ErrorTypeFatal // Unauthorized, forbidden, not found
		case 429, 500, 502, 503, 504:
			return ErrorTypeTransient // Rate limit, server errors
		default:
			return ErrorTypeFatal // Other HTTP errors
		}
	}

	// Network errors are transient
	return ErrorTypeTransient
}

// IsTransientError returns true if the error should be retried
func IsTransientError(err error) bool {
	return ClassifyError(err) == ErrorTypeTransient
}

// IsFatalError returns true if the error is permanent
func IsFatalError(err error) bool {
	return ClassifyError(err) == ErrorTypeFatal
}

// ParseTimestampUsec converts a timestampUsec string to time.Time
func ParseTimestampUsec(timestampUsec string) (time.Time, error) {
	var usec int64
	_, err := fmt.Sscanf(timestampUsec, "%d", &usec)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp: %w", err)
	}

	// Convert microseconds to seconds and nanoseconds
	sec := usec / 1000000
	nsec := (usec % 1000000) * 1000

	return time.Unix(sec, nsec).UTC(), nil
}
