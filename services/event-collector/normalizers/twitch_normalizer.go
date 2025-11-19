package normalizers

import (
	"time"

	"github.com/caesar/all-chat/services/event-collector/models"
	"github.com/google/uuid"
)

// TwitchNormalizer converts Twitch EventSub events to unified StreamEvent format
type TwitchNormalizer struct{}

// NewTwitchNormalizer creates a new Twitch normalizer
func NewTwitchNormalizer() *TwitchNormalizer {
	return &TwitchNormalizer{}
}

// TwitchFollowEvent represents a Twitch channel.follow EventSub notification
type TwitchFollowEvent struct {
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	FollowedAt   time.Time `json:"followed_at"`
}

// TwitchSubscribeEvent represents a Twitch channel.subscribe EventSub notification
type TwitchSubscribeEvent struct {
	UserID    string `json:"user_id"`
	UserLogin string `json:"user_login"`
	UserName  string `json:"user_name"`
	Tier      string `json:"tier"` // "1000", "2000", "3000"
	IsGift    bool   `json:"is_gift"`
}

// TwitchSubscriptionMessageEvent represents a Twitch channel.subscription.message EventSub notification
type TwitchSubscriptionMessageEvent struct {
	UserID               string `json:"user_id"`
	UserLogin            string `json:"user_login"`
	UserName             string `json:"user_name"`
	Tier                 string `json:"tier"`
	Message              string `json:"message"`
	CumulativeMonths     int    `json:"cumulative_months"`
	StreakMonths         *int   `json:"streak_months"`
	DurationMonths       int    `json:"duration_months"`
}

// TwitchSubscriptionGiftEvent represents a Twitch channel.subscription.gift EventSub notification
type TwitchSubscriptionGiftEvent struct {
	UserID           string `json:"user_id"`
	UserLogin        string `json:"user_login"`
	UserName         string `json:"user_name"`
	Tier             string `json:"tier"`
	Total            int    `json:"total"` // Number of gifted subs
	CumulativeTotal  *int   `json:"cumulative_total"`
	IsAnonymous      bool   `json:"is_anonymous"`
}

// TwitchCheerEvent represents a Twitch channel.cheer EventSub notification
type TwitchCheerEvent struct {
	UserID      string `json:"user_id"`
	UserLogin   string `json:"user_login"`
	UserName    string `json:"user_name"`
	Bits        int    `json:"bits"`
	Message     string `json:"message"`
	IsAnonymous bool   `json:"is_anonymous"`
}

// TwitchRaidEvent represents a Twitch channel.raid EventSub notification
type TwitchRaidEvent struct {
	FromBroadcasterUserID    string `json:"from_broadcaster_user_id"`
	FromBroadcasterUserLogin string `json:"from_broadcaster_user_login"`
	FromBroadcasterUserName  string `json:"from_broadcaster_user_name"`
	ToBroadcasterUserID      string `json:"to_broadcaster_user_id"`
	ToBroadcasterUserLogin   string `json:"to_broadcaster_user_login"`
	ToBroadcasterUserName    string `json:"to_broadcaster_user_name"`
	Viewers                  int    `json:"viewers"`
}

// NormalizeFollow converts a Twitch follow event to unified format
func (n *TwitchNormalizer) NormalizeFollow(
	event *TwitchFollowEvent,
	sessionID uuid.UUID,
	userID uuid.UUID,
) *models.StreamEvent {
	return &models.StreamEvent{
		ID:              uuid.New(),
		StreamSessionID: sessionID,
		UserID:          userID,
		Platform:        models.PlatformTwitch,
		EventType:       models.EventTypeFollow,
		EventSubtype:    nil,
		PlatformUser: models.PlatformUser{
			ID:          event.UserID,
			Username:    event.UserLogin,
			DisplayName: event.UserName,
			AvatarURL:   nil, // Twitch EventSub doesn't include avatar
		},
		Metadata:     make(map[string]interface{}),
		OccurredAt:   event.FollowedAt,
		CreatedAt:    time.Now(),
		IsTest:       false,
		IsBackfilled: false,
	}
}

// NormalizeSubscribe converts a Twitch subscribe event to unified format
func (n *TwitchNormalizer) NormalizeSubscribe(
	event *TwitchSubscribeEvent,
	sessionID uuid.UUID,
	userID uuid.UUID,
) *models.StreamEvent {
	subtype := models.SubtypeNewSub
	metadata := map[string]interface{}{
		models.MetadataTier: event.Tier,
	}

	return &models.StreamEvent{
		ID:              uuid.New(),
		StreamSessionID: sessionID,
		UserID:          userID,
		Platform:        models.PlatformTwitch,
		EventType:       models.EventTypeSub,
		EventSubtype:    &subtype,
		PlatformUser: models.PlatformUser{
			ID:          event.UserID,
			Username:    event.UserLogin,
			DisplayName: event.UserName,
			AvatarURL:   nil,
		},
		Metadata:     metadata,
		OccurredAt:   time.Now(),
		CreatedAt:    time.Now(),
		IsTest:       false,
		IsBackfilled: false,
	}
}

// NormalizeSubscriptionMessage converts a Twitch resub message to unified format
func (n *TwitchNormalizer) NormalizeSubscriptionMessage(
	event *TwitchSubscriptionMessageEvent,
	sessionID uuid.UUID,
	userID uuid.UUID,
) *models.StreamEvent {
	subtype := models.SubtypeResub
	metadata := map[string]interface{}{
		models.MetadataTier:    event.Tier,
		models.MetadataMonths:  event.CumulativeMonths,
		models.MetadataMessage: event.Message,
	}

	if event.StreakMonths != nil {
		metadata[models.MetadataStreak] = *event.StreakMonths
	}

	return &models.StreamEvent{
		ID:              uuid.New(),
		StreamSessionID: sessionID,
		UserID:          userID,
		Platform:        models.PlatformTwitch,
		EventType:       models.EventTypeSub,
		EventSubtype:    &subtype,
		PlatformUser: models.PlatformUser{
			ID:          event.UserID,
			Username:    event.UserLogin,
			DisplayName: event.UserName,
			AvatarURL:   nil,
		},
		Metadata:     metadata,
		OccurredAt:   time.Now(),
		CreatedAt:    time.Now(),
		IsTest:       false,
		IsBackfilled: false,
	}
}

// NormalizeGiftSub converts a Twitch gift sub event to unified format
func (n *TwitchNormalizer) NormalizeGiftSub(
	event *TwitchSubscriptionGiftEvent,
	sessionID uuid.UUID,
	userID uuid.UUID,
) *models.StreamEvent {
	displayName := event.UserName
	if event.IsAnonymous {
		displayName = "Anonymous"
	}

	metadata := map[string]interface{}{
		models.MetadataTier:           event.Tier,
		models.MetadataRecipientCount: event.Total,
	}

	if event.CumulativeTotal != nil {
		metadata["cumulative_total"] = *event.CumulativeTotal
	}

	return &models.StreamEvent{
		ID:              uuid.New(),
		StreamSessionID: sessionID,
		UserID:          userID,
		Platform:        models.PlatformTwitch,
		EventType:       models.EventTypeGiftSub,
		EventSubtype:    nil,
		PlatformUser: models.PlatformUser{
			ID:          event.UserID,
			Username:    event.UserLogin,
			DisplayName: displayName,
			AvatarURL:   nil,
		},
		Metadata:     metadata,
		OccurredAt:   time.Now(),
		CreatedAt:    time.Now(),
		IsTest:       false,
		IsBackfilled: false,
	}
}

// NormalizeCheer converts a Twitch cheer event to unified format
func (n *TwitchNormalizer) NormalizeCheer(
	event *TwitchCheerEvent,
	sessionID uuid.UUID,
	userID uuid.UUID,
) *models.StreamEvent {
	displayName := event.UserName
	if event.IsAnonymous {
		displayName = "Anonymous"
	}

	metadata := map[string]interface{}{
		models.MetadataAmount:  event.Bits,
		models.MetadataMessage: event.Message,
	}

	return &models.StreamEvent{
		ID:              uuid.New(),
		StreamSessionID: sessionID,
		UserID:          userID,
		Platform:        models.PlatformTwitch,
		EventType:       models.EventTypeBits,
		EventSubtype:    nil,
		PlatformUser: models.PlatformUser{
			ID:          event.UserID,
			Username:    event.UserLogin,
			DisplayName: displayName,
			AvatarURL:   nil,
		},
		Metadata:     metadata,
		OccurredAt:   time.Now(),
		CreatedAt:    time.Now(),
		IsTest:       false,
		IsBackfilled: false,
	}
}

// NormalizeRaid converts a Twitch raid event to unified format
func (n *TwitchNormalizer) NormalizeRaid(
	event *TwitchRaidEvent,
	sessionID uuid.UUID,
	userID uuid.UUID,
) *models.StreamEvent {
	metadata := map[string]interface{}{
		models.MetadataRaidViewerCount: event.Viewers,
		models.MetadataFromBroadcaster:  event.FromBroadcasterUserName,
	}

	return &models.StreamEvent{
		ID:              uuid.New(),
		StreamSessionID: sessionID,
		UserID:          userID,
		Platform:        models.PlatformTwitch,
		EventType:       models.EventTypeRaid,
		EventSubtype:    nil,
		PlatformUser: models.PlatformUser{
			ID:          event.FromBroadcasterUserID,
			Username:    event.FromBroadcasterUserLogin,
			DisplayName: event.FromBroadcasterUserName,
			AvatarURL:   nil,
		},
		Metadata:     metadata,
		OccurredAt:   time.Now(),
		CreatedAt:    time.Now(),
		IsTest:       false,
		IsBackfilled: false,
	}
}
