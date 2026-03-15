package gateway

import "encoding/json"

// Op codes
const (
	OpDispatch       = 0
	OpHeartbeat      = 1
	OpIdentify       = 2
	OpReconnect      = 7
	OpInvalidSession = 9
	OpHello          = 10
	OpHeartbeatACK   = 11
)

// Intent bitmask values
const (
	IntentGuilds         = 1 << 0  // 1
	IntentGuildMessages  = 1 << 9  // 512
	IntentMessageContent = 1 << 15 // 32768 — PRIVILEGED, must be enabled in Discord Developer Portal
	RequiredIntents      = IntentGuilds | IntentGuildMessages | IntentMessageContent // 33281
)

// Redis key schema for Gateway session state (shard 0)
const (
	RedisKeySessionID = "discord:gateway:shard:0:session_id"
	RedisKeyResumeURL = "discord:gateway:shard:0:resume_url"
	RedisKeySeq       = "discord:gateway:shard:0:seq"
)

// GatewayPayload is the base WebSocket message envelope
type GatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *int            `json:"s,omitempty"` // sequence number (dispatch only)
	T  *string         `json:"t,omitempty"` // event name (dispatch only)
}

// HelloData is the payload for op=10 HELLO
type HelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// IdentifyData is the payload for op=2 IDENTIFY
type IdentifyData struct {
	Token      string             `json:"token"`
	Intents    int                `json:"intents"`
	Properties IdentifyProperties `json:"properties"`
}

// IdentifyProperties describes the client to Discord
type IdentifyProperties struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

// ReadyEventData is the d field of the READY dispatch event
type ReadyEventData struct {
	SessionID        string `json:"session_id"`
	ResumeGatewayURL string `json:"resume_gateway_url"`
}

// GatewaySession holds the resumable session state
type GatewaySession struct {
	SessionID string
	ResumeURL string
	Seq       int
}

// MessageCreateData is the payload for the MESSAGE_CREATE dispatch event
type MessageCreateData struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id"`
	GuildID   string         `json:"guild_id"`
	Content   string         `json:"content"`
	Timestamp string         `json:"timestamp"`
	Author    DiscordUser    `json:"author"`
	Member    *DiscordMember `json:"member"`
}

// DiscordUser represents the author of a Discord message
type DiscordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Bot        bool   `json:"bot"`
}

// DiscordMember holds the guild-specific member data attached to a MESSAGE_CREATE event
type DiscordMember struct {
	Nick  *string  `json:"nick"`
	Roles []string `json:"roles"`
}
