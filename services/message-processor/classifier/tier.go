package classifier

import (
	"github.com/caesar/all-chat/services/message-processor/models"
)

// ClassifyEvent determines the tier (high/medium/low) and display duration for an event
// based on platform, event type, and value
func ClassifyEvent(platform, eventType string, value *models.EventValue) (tier string, duration int) {
	// Default to medium tier with 15 second duration
	tier = "medium"
	duration = 15

	switch platform {
	case "twitch":
		return classifyTwitchEvent(eventType, value)
	case "youtube":
		return classifyYouTubeEvent(eventType, value)
	case "kick":
		return classifyKickEvent(eventType, value)
	case "tiktok":
		return classifyTikTokEvent(eventType, value)
	default:
		return tier, duration
	}
}

func classifyTwitchEvent(eventType string, value *models.EventValue) (tier string, duration int) {
	switch eventType {
	case "subscription", "resubscription":
		// Subscriptions are high value
		return "high", 30

	case "gift_subscription":
		// Gift subs are high value
		return "high", 30

	case "mystery_gift":
		// Mystery gifts are high value (multiple subs at once)
		return "high", 45

	case "raid":
		// Raids: tier based on viewer count
		if value != nil && value.Amount >= 1000 {
			return "high", 40 // Large raid (1000+ viewers)
		} else if value != nil && value.Amount >= 100 {
			return "high", 30 // Medium raid (100-999 viewers)
		} else {
			return "medium", 20 // Small raid (<100 viewers)
		}

	case "bits":
		// Bits: tier based on amount
		if value != nil && value.Amount >= 1000 {
			return "high", 35 // 1000+ bits
		} else if value != nil && value.Amount >= 100 {
			return "medium", 20 // 100-999 bits
		} else {
			return "low", 10 // <100 bits
		}

	case "channel_points":
		// Channel point redemptions are medium value
		return "medium", 15

	case "ritual":
		// First-time chatter ritual is low value
		return "low", 10

	default:
		return "medium", 15
	}
}

func classifyYouTubeEvent(eventType string, value *models.EventValue) (tier string, duration int) {
	switch eventType {
	case "super_chat":
		// Super Chat: tier based on amount in micros (1,000,000 micros = $1)
		if value != nil && value.Amount >= 50000000 {
			return "high", 60 // $50+
		} else if value != nil && value.Amount >= 10000000 {
			return "high", 45 // $10-$49
		} else if value != nil && value.Amount >= 5000000 {
			return "high", 30 // $5-$9
		} else if value != nil && value.Amount >= 2000000 {
			return "medium", 20 // $2-$4
		} else {
			return "low", 10 // <$2
		}

	case "super_sticker":
		// Super Stickers: similar to Super Chat
		if value != nil && value.Amount >= 10000000 {
			return "high", 40 // $10+
		} else if value != nil && value.Amount >= 5000000 {
			return "medium", 25 // $5-$9
		} else {
			return "low", 12 // <$5
		}

	case "new_sponsor":
		// New membership is high value
		return "high", 30

	case "member_milestone":
		// Member milestones: tier based on months
		if value != nil && value.Amount >= 24 {
			return "high", 35 // 2+ years
		} else if value != nil && value.Amount >= 12 {
			return "high", 30 // 1+ year
		} else if value != nil && value.Amount >= 6 {
			return "medium", 20 // 6+ months
		} else {
			return "medium", 15 // <6 months
		}

	case "membership_gift":
		// Membership gifting: tier based on gift count
		if value != nil && value.Amount >= 10 {
			return "high", 40 // 10+ gifts
		} else if value != nil && value.Amount >= 5 {
			return "high", 30 // 5-9 gifts
		} else {
			return "medium", 20 // <5 gifts
		}

	case "gift_received":
		// Receiving a gift membership is medium value
		return "medium", 15

	case "message_deleted", "user_banned":
		// Moderation events are low priority
		return "low", 8

	default:
		return "medium", 15
	}
}

func classifyKickEvent(eventType string, value *models.EventValue) (tier string, duration int) {
	switch eventType {
	case "subscription":
		// Subscriptions are high value
		return "high", 30

	case "gift_subscription":
		// Gift subs are high value
		return "high", 30

	case "donation":
		// Donations: tier based on amount
		if value != nil && value.Amount >= 50 {
			return "high", 45 // $50+
		} else if value != nil && value.Amount >= 10 {
			return "high", 30 // $10-$49
		} else if value != nil && value.Amount >= 5 {
			return "medium", 20 // $5-$9
		} else {
			return "low", 10 // <$5
		}

	default:
		return "medium", 15
	}
}

func classifyTikTokEvent(eventType string, value *models.EventValue) (tier string, duration int) {
	switch eventType {
	case "gift":
		// Gifts: tier based on diamond count
		if value != nil && value.Amount >= 1000 {
			return "high", 35 // 1000+ diamonds
		} else if value != nil && value.Amount >= 100 {
			return "medium", 20 // 100-999 diamonds
		} else {
			return "low", 10 // <100 diamonds
		}

	case "follow":
		// Follows are medium value
		return "medium", 15

	case "like_aggregate":
		// Like aggregates are low value (common/spammy)
		if value != nil && value.Amount >= 100 {
			return "medium", 12 // 100+ likes in window
		} else {
			return "low", 8 // <100 likes
		}

	case "share":
		// Shares are medium value
		return "medium", 15

	default:
		return "medium", 15
	}
}
