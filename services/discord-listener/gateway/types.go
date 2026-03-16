package gateway

import "encoding/json"

// Op codes
const (
	OpDispatch       = 0
	OpHeartbeat      = 1
	OpIdentify       = 2
	OpResume         = 6 // client sends to resume a disconnected session
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

// ResumeData is the payload for op=6 RESUME
type ResumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}

// BuildResumePayload constructs the op=6 RESUME payload.
func BuildResumePayload(token, sessionID string, seq int) GatewayPayload {
	d, _ := json.Marshal(ResumeData{
		Token:     token,
		SessionID: sessionID,
		Seq:       seq,
	})
	return GatewayPayload{Op: OpResume, D: json.RawMessage(d)}
}

// InvalidSessionData is the payload for op=9 INVALID_SESSION.
// Resumable is true when the client may attempt RESUME on reconnect; false means IDENTIFY required.
type InvalidSessionData struct {
	Resumable bool `json:"d"`
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
	Mentions  []DiscordUser  `json:"mentions"`
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

// MessageDeleteData is the payload for the MESSAGE_DELETE dispatch event
type MessageDeleteData struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
}

// MessageDeleteBulkData is the payload for the MESSAGE_DELETE_BULK dispatch event
type MessageDeleteBulkData struct {
	IDs       []string `json:"ids"`
	ChannelID string   `json:"channel_id"`
	GuildID   string   `json:"guild_id"`
}

// GuildCreateData is the payload for the GUILD_CREATE dispatch event
type GuildCreateData struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Channels []DiscordChannel `json:"channels"`
	Roles    []DiscordRole    `json:"roles"`
}

// DiscordChannel represents a Discord channel (from GUILD_CREATE or CHANNEL_* events)
type DiscordChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type int    `json:"type"` // 0 = GUILD_TEXT; useful to filter to text channels
}

// DiscordRole represents a Discord role (from GUILD_CREATE or GUILD_ROLE_* events)
type DiscordRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ChannelUpdateData is the payload for CHANNEL_UPDATE, CHANNEL_CREATE, and CHANNEL_DELETE events
type ChannelUpdateData struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	GuildID string `json:"guild_id"`
}

// GuildRoleUpdateData is the payload for GUILD_ROLE_UPDATE and GUILD_ROLE_CREATE events
type GuildRoleUpdateData struct {
	GuildID string      `json:"guild_id"`
	Role    DiscordRole `json:"role"`
}

// GuildRoleDeleteData is the payload for GUILD_ROLE_DELETE events
type GuildRoleDeleteData struct {
	GuildID string `json:"guild_id"`
	RoleID  string `json:"role_id"`
}
