// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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

// ResubscriptionEvent represents a resubscription message event
// subscription type: channel.subscription.message
type ResubscriptionEvent struct {
	UserID               string             `json:"user_id"`
	UserLogin            string             `json:"user_login"`
	UserName             string             `json:"user_name"`
	BroadcasterUserID    string             `json:"broadcaster_user_id"`
	BroadcasterUserLogin string             `json:"broadcaster_user_login"`
	BroadcasterUserName  string             `json:"broadcaster_user_name"`
	Tier                 string             `json:"tier"`
	Message              ResubscriptionMessage `json:"message"`
	CumulativeMonths     int                `json:"cumulative_months"`
	StreakMonths         int                `json:"streak_months"`
	DurationMonths       int                `json:"duration_months"`
}

// ResubscriptionMessage contains the resub message
type ResubscriptionMessage struct {
	Text   string        `json:"text"`
	Emotes []interface{} `json:"emotes"`
}

// RaidEvent represents a raid event
// subscription type: channel.raid
type RaidEvent struct {
	FromBroadcasterUserID    string    `json:"from_broadcaster_user_id"`
	FromBroadcasterUserLogin string    `json:"from_broadcaster_user_login"`
	FromBroadcasterUserName  string    `json:"from_broadcaster_user_name"`
	ToBroadcasterUserID      string    `json:"to_broadcaster_user_id"`
	ToBroadcasterUserLogin   string    `json:"to_broadcaster_user_login"`
	ToBroadcasterUserName    string    `json:"to_broadcaster_user_name"`
	Viewers                  int       `json:"viewers"`
}

// CheerEvent represents a bits/cheer event
// subscription type: channel.cheer
type CheerEvent struct {
	IsAnonymous          bool   `json:"is_anonymous"`
	UserID               string `json:"user_id"`
	UserLogin            string `json:"user_login"`
	UserName             string `json:"user_name"`
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	Message              string `json:"message"`
	Bits                 int    `json:"bits"`
}

// FollowEvent represents a follow event
// subscription type: channel.follow (v2)
type FollowEvent struct {
	UserID               string    `json:"user_id"`
	UserLogin            string    `json:"user_login"`
	UserName             string    `json:"user_name"`
	BroadcasterUserID    string    `json:"broadcaster_user_id"`
	BroadcasterUserLogin string    `json:"broadcaster_user_login"`
	BroadcasterUserName  string    `json:"broadcaster_user_name"`
	FollowedAt           time.Time `json:"followed_at"`
}

// ChatMessageEvent represents a chat message event.
// subscription type: channel.chat.message (v1)
// Reference: https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchatmessage
type ChatMessageEvent struct {
	BroadcasterUserID    string          `json:"broadcaster_user_id"`
	BroadcasterUserLogin string          `json:"broadcaster_user_login"`
	BroadcasterUserName  string          `json:"broadcaster_user_name"`
	ChatterUserID        string          `json:"chatter_user_id"`
	ChatterUserLogin     string          `json:"chatter_user_login"`
	ChatterUserName      string          `json:"chatter_user_name"`
	MessageID            string          `json:"message_id"`
	Message              ChatMessageBody `json:"message"`
	Color                string          `json:"color"` // hex, may be ""
	Badges               []ChatBadge     `json:"badges"`
	MessageType          string          `json:"message_type"` // "text", "channel_points_highlighted", "power_ups_message_effect", ...
	Cheer                *ChatCheer      `json:"cheer,omitempty"`
	Reply                *ChatReply      `json:"reply,omitempty"`

	// Shared-chat fields — present only during a shared-chat session.
	SourceBroadcasterUserID    string      `json:"source_broadcaster_user_id,omitempty"`
	SourceBroadcasterUserLogin string      `json:"source_broadcaster_user_login,omitempty"`
	SourceBroadcasterUserName  string      `json:"source_broadcaster_user_name,omitempty"`
	SourceMessageID            string      `json:"source_message_id,omitempty"`
	SourceBadges               []ChatBadge `json:"source_badges,omitempty"`
}

// ChatMessageBody is the structured message of a channel.chat.message event.
type ChatMessageBody struct {
	Text      string                `json:"text"`
	Fragments []ChatMessageFragment `json:"fragments"`
}

// ChatMessageFragment is one piece of a chat message. Type is one of
// "text", "cheermote", "emote", "mention".
type ChatMessageFragment struct {
	Type      string         `json:"type"`
	Text      string         `json:"text"`
	Cheermote *ChatCheermote `json:"cheermote,omitempty"`
	Emote     *ChatEmote     `json:"emote,omitempty"`
	Mention   *ChatMention   `json:"mention,omitempty"`
}

// ChatEmote describes a first-party Twitch emote in a message fragment.
type ChatEmote struct {
	ID         string   `json:"id"`
	EmoteSetID string   `json:"emote_set_id"`
	OwnerID    string   `json:"owner_id"`
	Format     []string `json:"format"` // "static", "animated"
}

// ChatCheermote describes a cheermote (bits) fragment.
type ChatCheermote struct {
	Prefix string `json:"prefix"`
	Bits   int    `json:"bits"`
	Tier   int    `json:"tier"`
}

// ChatMention describes an @-mention fragment.
type ChatMention struct {
	UserID    string `json:"user_id"`
	UserLogin string `json:"user_login"`
	UserName  string `json:"user_name"`
}

// ChatBadge is a chat badge on a channel.chat.message event. Unlike IRC tags, the
// badge version lives in ID (e.g. set_id "subscriber", id "12"); Info carries extra
// context (e.g. months subscribed) for badges that have it.
type ChatBadge struct {
	SetID string `json:"set_id"`
	ID    string `json:"id"`
	Info  string `json:"info"`
}

// ChatCheer is the top-level cheer (bits) total on a chat message, when present.
type ChatCheer struct {
	Bits int `json:"bits"`
}

// ChatReply describes reply threading metadata, when the message is a reply.
type ChatReply struct {
	ParentMessageID   string `json:"parent_message_id"`
	ParentMessageBody string `json:"parent_message_body"`
	ParentUserID      string `json:"parent_user_id"`
	ParentUserLogin   string `json:"parent_user_login"`
	ParentUserName    string `json:"parent_user_name"`
	ThreadMessageID   string `json:"thread_message_id"`
	ThreadUserID      string `json:"thread_user_id"`
	ThreadUserLogin   string `json:"thread_user_login"`
	ThreadUserName    string `json:"thread_user_name"`
}

// ChatMessageDeleteEvent represents a single-message deletion (a moderator removed one message).
// subscription type: channel.chat.message_delete (v1)
// Reference: https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchatmessage_delete
// MessageID is the native Twitch id of the removed message — the same value the chat path stamps
// into Tags["id"], so it resolves to the displayed message via the message-ID registry.
type ChatMessageDeleteEvent struct {
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	TargetUserID         string `json:"target_user_id"`
	TargetUserLogin      string `json:"target_user_login"`
	TargetUserName       string `json:"target_user_name"`
	MessageID            string `json:"message_id"`
}

// ChatClearUserMessagesEvent represents removing all of one user's messages (a timeout or ban).
// subscription type: channel.chat.clear_user_messages (v1)
// Reference: https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchatclear_user_messages
// NOTE: Twitch does not include a duration here, so a timeout cannot be distinguished from a
// permanent ban from this event (unlike IRC's CLEARCHAT @ban-duration tag). Downstream treats the
// absence of a duration as a ban; either way every message from the user is removed.
type ChatClearUserMessagesEvent struct {
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	TargetUserID         string `json:"target_user_id"`
	TargetUserLogin      string `json:"target_user_login"`
	TargetUserName       string `json:"target_user_name"`
}

// ChatClearEvent represents clearing the entire chat.
// subscription type: channel.chat.clear (v1)
// Reference: https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelchatclear
type ChatClearEvent struct {
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
}

// SubscriptionInfo contains subscription metadata (used in webhook callbacks)
type SubscriptionInfo struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"`
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Condition map[string]interface{} `json:"condition"`
	Transport map[string]interface{} `json:"transport"`
	CreatedAt time.Time              `json:"created_at"`
	Cost      int                    `json:"cost"`
}
