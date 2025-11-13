package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/google/uuid"
	"google.golang.org/api/youtube/v3"
)

// Parser parses YouTube API responses into our internal models
type Parser struct{}

// NewParser creates a new YouTube API response parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseChatMessage converts a YouTube LiveChatMessage to our RawChatMessage format
func (p *Parser) ParseChatMessage(msg *youtube.LiveChatMessage, channelID, streamID string) (*models.RawChatMessage, error) {
	if msg.Snippet == nil || msg.AuthorDetails == nil {
		return nil, fmt.Errorf("invalid message: missing snippet or author details")
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, msg.Snippet.PublishedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Extract message text
	text := ""
	if msg.Snippet.TextMessageDetails != nil {
		text = msg.Snippet.TextMessageDetails.MessageText
	} else if msg.Snippet.SuperChatDetails != nil {
		text = msg.Snippet.SuperChatDetails.UserComment
	} else if msg.Snippet.SuperStickerDetails != nil {
		text = msg.Snippet.SuperStickerDetails.SuperStickerMetadata.AltText
	}

	// Build tags map with YouTube-specific metadata
	tags := make(map[string]string)
	tags["channel_id"] = msg.AuthorDetails.ChannelId
	tags["channel_url"] = msg.AuthorDetails.ChannelUrl
	tags["display_name"] = msg.AuthorDetails.DisplayName

	if msg.AuthorDetails.ProfileImageUrl != "" {
		tags["profile_image"] = msg.AuthorDetails.ProfileImageUrl
	}

	tags["is_verified"] = strconv.FormatBool(msg.AuthorDetails.IsVerified)
	tags["is_owner"] = strconv.FormatBool(msg.AuthorDetails.IsChatOwner)
	tags["is_sponsor"] = strconv.FormatBool(msg.AuthorDetails.IsChatSponsor)
	tags["is_moderator"] = strconv.FormatBool(msg.AuthorDetails.IsChatModerator)

	// Super Chat / Super Sticker amounts
	superChatAmount := int64(0)
	superStickerAmount := int64(0)

	if msg.Snippet.SuperChatDetails != nil {
		superChatAmount = int64(msg.Snippet.SuperChatDetails.AmountMicros)
		tags["super_chat_currency"] = msg.Snippet.SuperChatDetails.Currency
		tags["super_chat_display"] = msg.Snippet.SuperChatDetails.AmountDisplayString
	}

	if msg.Snippet.SuperStickerDetails != nil {
		superStickerAmount = int64(msg.Snippet.SuperStickerDetails.AmountMicros)
		tags["super_sticker_currency"] = msg.Snippet.SuperStickerDetails.Currency
		tags["super_sticker_display"] = msg.Snippet.SuperStickerDetails.AmountDisplayString
		tags["super_sticker_tier"] = strconv.FormatInt(msg.Snippet.SuperStickerDetails.Tier, 10)
	}

	tags["super_chat"] = strconv.FormatInt(superChatAmount, 10)
	tags["super_sticker"] = strconv.FormatInt(superStickerAmount, 10)

	// Create RawChatMessage
	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  streamID,
		UserID:    msg.AuthorDetails.ChannelId,
		Username:  msg.AuthorDetails.DisplayName,
		Text:      text,
		Timestamp: timestamp,
		Tags:      tags,
	}

	return rawMsg, nil
}

// ParseBatch parses multiple chat messages
func (p *Parser) ParseBatch(messages []*youtube.LiveChatMessage, channelID, streamID string) ([]*models.RawChatMessage, error) {
	result := make([]*models.RawChatMessage, 0, len(messages))

	for _, msg := range messages {
		rawMsg, err := p.ParseChatMessage(msg, channelID, streamID)
		if err != nil {
			// Log error but continue processing other messages
			continue
		}
		result = append(result, rawMsg)
	}

	return result, nil
}

// ExtractPollingInterval extracts the recommended polling interval from API response
func (p *Parser) ExtractPollingInterval(response *youtube.LiveChatMessageListResponse) int {
	if response.PollingIntervalMillis > 0 {
		return int(response.PollingIntervalMillis)
	}
	// Default to 5 seconds if not provided
	return 5000
}

// ExtractNextPageToken extracts the next page token from API response
func (p *Parser) ExtractNextPageToken(response *youtube.LiveChatMessageListResponse) string {
	return response.NextPageToken
}
