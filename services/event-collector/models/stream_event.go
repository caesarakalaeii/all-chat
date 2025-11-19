package models

import (
	"time"

	"github.com/google/uuid"
)

// StreamEvent represents a unified event from any platform
type StreamEvent struct {
	ID              uuid.UUID              `json:"id"`
	StreamSessionID uuid.UUID              `json:"stream_session_id"`
	UserID          uuid.UUID              `json:"user_id"`
	Platform        string                 `json:"platform"` // twitch, youtube, kick, tiktok
	EventType       string                 `json:"event_type"` // follow, sub, bits, raid, gift_sub, super_chat, chatter, etc.
	EventSubtype    *string                `json:"event_subtype,omitempty"` // new_sub, resub, tier_1, tier_2, tier_3, etc.
	PlatformUser    PlatformUser           `json:"platform_user"`
	Metadata        map[string]interface{} `json:"metadata"`
	OccurredAt      time.Time              `json:"occurred_at"`
	CreatedAt       time.Time              `json:"created_at"`
	IsTest          bool                   `json:"is_test"`
	IsBackfilled    bool                   `json:"is_backfilled"`
}

// PlatformUser represents the user who triggered the event
type PlatformUser struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

// Platform constants
const (
	PlatformTwitch  = "twitch"
	PlatformYouTube = "youtube"
	PlatformKick    = "kick"
	PlatformTikTok  = "tiktok"
)

// Event type constants
const (
	EventTypeFollow      = "follow"
	EventTypeSub         = "sub"
	EventTypeGiftSub     = "gift_sub"
	EventTypeBits        = "bits"
	EventTypeRaid        = "raid"
	EventTypeSuperChat   = "super_chat"
	EventTypeChannelPoints = "channel_points"
	EventTypeChatter     = "chatter"
	EventTypeMembership  = "membership"
	EventTypeTikTokGift  = "tiktok_gift"
	EventTypeHypeTrain   = "hype_train"
)

// Event subtype constants
const (
	SubtypeNewSub  = "new_sub"
	SubtypeResub   = "resub"
	SubtypeTier1   = "tier_1"
	SubtypeTier2   = "tier_2"
	SubtypeTier3   = "tier_3"
)

// Metadata keys for different event types
const (
	// For bits/super chats
	MetadataAmount   = "amount"
	MetadataCurrency = "currency"
	MetadataMessage  = "message"

	// For subscriptions
	MetadataTier          = "tier"
	MetadataMonths        = "months"
	MetadataStreak        = "streak"
	MetadataRecipientCount = "recipient_count"
	MetadataRecipients    = "recipients"

	// For raids
	MetadataRaidViewerCount = "raid_viewer_count"
	MetadataFromBroadcaster = "from_broadcaster"

	// For channel points
	MetadataRewardTitle = "reward_title"
	MetadataRewardCost  = "reward_cost"

	// For TikTok gifts
	MetadataGiftType  = "gift_type"
	MetadataGiftCount = "gift_count"
)
