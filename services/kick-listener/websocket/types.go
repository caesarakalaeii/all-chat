package websocket

import "encoding/json"

// PusherMessage represents a message received from Pusher WebSocket
type PusherMessage struct {
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data,omitempty"`
	Channel string          `json:"channel,omitempty"`
}

// PusherConnectionEstablished is sent when connection is established
type PusherConnectionEstablished struct {
	SocketID        string `json:"socket_id"`
	ActivityTimeout int    `json:"activity_timeout"`
}

// PusherSubscribe is used to subscribe to a channel
type PusherSubscribe struct {
	Event string              `json:"event"`
	Data  PusherSubscribeData `json:"data"`
}

type PusherSubscribeData struct {
	Channel string  `json:"channel"`
	Auth    *string `json:"auth,omitempty"` // Required for private channels
}

// PusherUnsubscribe is used to unsubscribe from a channel
type PusherUnsubscribe struct {
	Event string                `json:"event"`
	Data  PusherUnsubscribeData `json:"data"`
}

type PusherUnsubscribeData struct {
	Channel string `json:"channel"`
}

// KickChatMessage represents a Kick chat message from the WebSocket
// Event: "App\\Events\\ChatMessageSentEvent"
type KickChatMessage struct {
	ID           string            `json:"id"`
	ChatroomID   int               `json:"chatroom_id"`
	Content      string            `json:"content"`
	Type         string            `json:"type"` // "message", "reply", etc.
	CreatedAt    string            `json:"created_at"`
	Sender       KickMessageSender `json:"sender"`
	MessageParts []KickMessagePart `json:"message_parts,omitempty"`
}

// KickMessageSender represents the sender of a Kick message
type KickMessageSender struct {
	ID       int                `json:"id"`
	Username string             `json:"username"`
	Slug     string             `json:"slug"`
	Identity KickSenderIdentity `json:"identity"`
}

// KickSenderIdentity represents badges and identity info
type KickSenderIdentity struct {
	Color  string      `json:"color"`
	Badges []KickBadge `json:"badges"`
}

// KickMessagePart represents segmented content such as emotes
type KickMessagePart struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	URL   string `json:"url,omitempty"`
}

// KickBadge represents a user badge
type KickBadge struct {
	Type string `json:"type"` // "subscriber", "moderator", "vip", etc.
	Text string `json:"text"`
}

// KickChannelInfo represents channel information from Kick API
type KickChannelInfo struct {
	ID          int              `json:"id"`
	UserID      int              `json:"user_id"`
	Slug        string           `json:"slug"`
	IsLive      bool             `json:"is_live"`
	PlaybackURL json.RawMessage  `json:"playback_url"` // Can be string or object
	Chatroom    KickChatroomInfo `json:"chatroom"`
}

// KickPlaybackInfo contains stream playback information
type KickPlaybackInfo struct {
	HLS string `json:"hls"`
}

// KickChatroomInfo contains chatroom information
type KickChatroomInfo struct {
	ID              int  `json:"id"`
	ChatroomID      int  `json:"chatroom_id"`
	ChannelID       int  `json:"channel_id"`
	SlowMode        bool `json:"slow_mode"`
	SubscribersMode bool `json:"subscribers_mode"`
	FollowersMode   bool `json:"followers_mode"`
}

// PusherErrorMessage represents an error from Pusher
type PusherErrorMessage struct {
	Event   string `json:"event"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// KickMessageDeletedEvent represents a deleted message event from Kick
// Event: "App\\Events\\ChatMessageDeletedEvent"
type KickMessageDeletedEvent struct {
	DeletedMessage struct {
		ID         string `json:"id"`          // Kick platform message ID
		DeletedBy  int    `json:"deleted_by"`  // User ID who deleted message
		ChatroomID string `json:"chatroom_id"` // Chatroom ID
	} `json:"deletedMessage"`
}
