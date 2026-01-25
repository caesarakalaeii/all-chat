package api

import (
	"fmt"
	"strconv"
	"strings"
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

	// Determine event type and extract message text and event data
	eventType := ""
	eventData := make(map[string]interface{})
	text := ""

	if msg.Snippet.TextMessageDetails != nil {
		// Regular chat message (default, no event type)
		text = msg.Snippet.TextMessageDetails.MessageText

	} else if msg.Snippet.SuperChatDetails != nil {
		// Super Chat event
		eventType = "super_chat"
		text = msg.Snippet.SuperChatDetails.UserComment
		eventData["amount_micros"] = msg.Snippet.SuperChatDetails.AmountMicros
		eventData["currency"] = msg.Snippet.SuperChatDetails.Currency
		eventData["amount_display"] = msg.Snippet.SuperChatDetails.AmountDisplayString
		eventData["tier"] = msg.Snippet.SuperChatDetails.Tier

	} else if msg.Snippet.SuperStickerDetails != nil {
		// Super Sticker event
		eventType = "super_sticker"
		text = msg.Snippet.SuperStickerDetails.SuperStickerMetadata.AltText
		eventData["amount_micros"] = msg.Snippet.SuperStickerDetails.AmountMicros
		eventData["currency"] = msg.Snippet.SuperStickerDetails.Currency
		eventData["amount_display"] = msg.Snippet.SuperStickerDetails.AmountDisplayString
		eventData["tier"] = msg.Snippet.SuperStickerDetails.Tier

	} else if msg.Snippet.NewSponsorDetails != nil {
		// New membership/sponsor event
		eventType = "new_sponsor"
		text = "Became a member"
		eventData["is_upgrade"] = msg.Snippet.NewSponsorDetails.IsUpgrade
		if msg.Snippet.NewSponsorDetails.MemberLevelName != "" {
			eventData["member_level_name"] = msg.Snippet.NewSponsorDetails.MemberLevelName
		}

	} else if msg.Snippet.MemberMilestoneChatDetails != nil {
		// Member milestone (anniversary) event
		eventType = "member_milestone"
		text = msg.Snippet.MemberMilestoneChatDetails.UserComment
		eventData["member_months"] = msg.Snippet.MemberMilestoneChatDetails.MemberMonth
		if msg.Snippet.MemberMilestoneChatDetails.MemberLevelName != "" {
			eventData["member_level_name"] = msg.Snippet.MemberMilestoneChatDetails.MemberLevelName
		}

	} else if msg.Snippet.MembershipGiftingDetails != nil {
		// Membership gifting event
		eventType = "membership_gift"
		giftCount := int(msg.Snippet.MembershipGiftingDetails.GiftMembershipsCount)
		text = fmt.Sprintf("Gifted %d memberships", giftCount)
		eventData["gift_count"] = giftCount
		if msg.Snippet.MembershipGiftingDetails.GiftMembershipsLevelName != "" {
			eventData["member_level_name"] = msg.Snippet.MembershipGiftingDetails.GiftMembershipsLevelName
		}

	} else if msg.Snippet.GiftMembershipReceivedDetails != nil {
		// Received gift membership event
		eventType = "gift_received"
		text = "Received a gift membership"
		eventData["gifter_channel_id"] = msg.Snippet.GiftMembershipReceivedDetails.GifterChannelId
		if msg.Snippet.GiftMembershipReceivedDetails.MemberLevelName != "" {
			eventData["member_level_name"] = msg.Snippet.GiftMembershipReceivedDetails.MemberLevelName
		}

	} else if msg.Snippet.MessageDeletedDetails != nil {
		// Message deleted (moderation) event
		eventType = "message_deleted"
		text = "Message deleted"
		eventData["deleted_message_id"] = msg.Snippet.MessageDeletedDetails.DeletedMessageId

	} else if msg.Snippet.UserBannedDetails != nil {
		// User banned (moderation) event
		eventType = "user_banned"
		text = "User banned"
		if msg.Snippet.UserBannedDetails.BannedUserDetails != nil {
			eventData["banned_user_id"] = msg.Snippet.UserBannedDetails.BannedUserDetails.ChannelId
			eventData["banned_user_name"] = msg.Snippet.UserBannedDetails.BannedUserDetails.DisplayName
		}
		eventData["ban_type"] = msg.Snippet.UserBannedDetails.BanType
		if msg.Snippet.UserBannedDetails.BanDurationSeconds > 0 {
			eventData["ban_duration_seconds"] = msg.Snippet.UserBannedDetails.BanDurationSeconds
		}
	}

	// Build tags map with YouTube-specific metadata
	tags := make(map[string]string)
	tags["channel_id"] = msg.AuthorDetails.ChannelId
	tags["channel_url"] = msg.AuthorDetails.ChannelUrl
	displayName := strings.TrimPrefix(msg.AuthorDetails.DisplayName, "@")
	tags["display_name"] = displayName

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

	// Create RawChatMessage with event fields
	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: channelID,
		StreamID:  streamID,
		UserID:    msg.AuthorDetails.ChannelId,
		Username:  displayName,
		Text:      text,
		Timestamp: timestamp,
		Tags:      tags,
		EventType: eventType,   // Empty for regular chat, populated for events
		EventData: eventData,   // Empty for regular chat, populated for events
	}

	return rawMsg, nil
}

// ParseBatch parses multiple chat messages
func (p *Parser) ParseBatch(messages []*youtube.LiveChatMessage, channelID, streamID string) ([]*models.RawChatMessage, error) {
	result := make([]*models.RawChatMessage, 0, len(messages))
	var skippedCount int

	for _, msg := range messages {
		rawMsg, err := p.ParseChatMessage(msg, channelID, streamID)
		if err != nil {
			// Log error but continue processing other messages
			skippedCount++
			continue
		}
		result = append(result, rawMsg)
	}

	// CRITICAL: Return error if ALL messages were skipped (indicates missing required parts like authorDetails)
	if len(messages) > 0 && len(result) == 0 {
		return result, fmt.Errorf("all %d messages failed to parse (skipped: %d) - likely missing required 'authorDetails' part in API request", len(messages), skippedCount)
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
