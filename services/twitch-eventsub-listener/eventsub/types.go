package eventsub

import "time"

// EventSub WebSocket Message Types
// Reference: https://dev.twitch.tv/docs/eventsub/handling-websocket-events/

// Message represents a WebSocket message from Twitch EventSub
type Message struct {
	Metadata MessageMetadata `json:"metadata"`
	Payload  Payload         `json:"payload"`
}

// MessageMetadata contains metadata about the message
type MessageMetadata struct {
	MessageID           string    `json:"message_id"`
	MessageType         string    `json:"message_type"` // "session_welcome", "notification", "session_keepalive", "session_reconnect"
	MessageTimestamp    time.Time `json:"message_timestamp"`
	SubscriptionType    string    `json:"subscription_type,omitempty"`
	SubscriptionVersion string    `json:"subscription_version,omitempty"`
}

// Payload contains the message payload
type Payload struct {
	Session      *Session      `json:"session,omitempty"`
	Subscription *Subscription `json:"subscription,omitempty"`
	Event        Event         `json:"event,omitempty"`
}

// Session contains session information
type Session struct {
	ID                      string    `json:"id"`
	Status                  string    `json:"status"`
	ConnectedAt             time.Time `json:"connected_at"`
	KeepaliveTimeoutSeconds int       `json:"keepalive_timeout_seconds"`
	ReconnectURL            string    `json:"reconnect_url,omitempty"`
}

// Subscription contains subscription information
type Subscription struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"`
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Condition map[string]interface{} `json:"condition"`
	Transport Transport              `json:"transport"`
	CreatedAt time.Time              `json:"created_at"`
	Cost      int                    `json:"cost"`
}

// Transport contains transport information
type Transport struct {
	Method    string `json:"method"`
	SessionID string `json:"session_id"`
}

// Event is a generic event payload (specific to subscription type)
type Event map[string]interface{}

// ChannelPointsRedemption represents a channel points redemption event
// subscription type: channel.channel_points_custom_reward_redemption.add
type ChannelPointsRedemption struct {
	ID                   string    `json:"id"`
	BroadcasterUserID    string    `json:"broadcaster_user_id"`
	BroadcasterUserLogin string    `json:"broadcaster_user_login"`
	BroadcasterUserName  string    `json:"broadcaster_user_name"`
	UserID               string    `json:"user_id"`
	UserLogin            string    `json:"user_login"`
	UserName             string    `json:"user_name"`
	UserInput            string    `json:"user_input"`
	Status               string    `json:"status"` // "unfulfilled", "fulfilled", "canceled"
	Reward               Reward    `json:"reward"`
	RedeemedAt           time.Time `json:"redeemed_at"`
}

// Reward represents a channel points reward
type Reward struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Cost   int    `json:"cost"`
	Prompt string `json:"prompt"`
}

// SubscribeEvent represents a subscription event
// subscription type: channel.subscribe
type SubscribeEvent struct {
	UserID               string `json:"user_id"`
	UserLogin            string `json:"user_login"`
	UserName             string `json:"user_name"`
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	Tier                 string `json:"tier"` // "1000", "2000", "3000"
	IsGift               bool   `json:"is_gift"`
}

// SubscriptionGiftEvent represents a subscription gift event
// subscription type: channel.subscription.gift
type SubscriptionGiftEvent struct {
	UserID               string `json:"user_id"`
	UserLogin            string `json:"user_login"`
	UserName             string `json:"user_name"`
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	Total                int    `json:"total"` // Number of subscriptions gifted
	Tier                 string `json:"tier"`
	CumulativeTotal      int    `json:"cumulative_total"` // Total gifts by this user
	IsAnonymous          bool   `json:"is_anonymous"`
}
