package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/discord-listener/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// SessionStore is the interface for persisting Gateway session state to Redis.
// Using an interface allows unit testing with MockRedis.
type SessionStore interface {
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
}

// ChannelRegistry provides channel-to-overlay mapping lookups.
// Implementations back this with Redis or an in-memory map.
type ChannelRegistry interface {
	// GetOverlayForChannel returns the overlayID for the given channelID.
	// Returns ("", false, nil) when the key is absent (not configured).
	GetOverlayForChannel(ctx context.Context, channelID string) (overlayID string, found bool, err error)
	// ListConfiguredChannels returns all configured channel_id → overlay_id mappings.
	ListConfiguredChannels(ctx context.Context) (map[string]string, error)
	// Subscribe registers a channel to receive channelID strings on invalidation events.
	Subscribe(ctx context.Context, ch chan<- string) error
}

// MessagePublisher publishes a raw message to downstream consumers (e.g. Redis Streams).
// The message argument is typed as interface{} so the gateway package does not import
// the publisher package, avoiding a circular dependency.
type MessagePublisher interface {
	Publish(ctx context.Context, msg interface{}) error
}

// DemandChecker checks if a Discord channel currently has overlay demand.
// When nil, all channels are considered to have demand (backward compat).
type DemandChecker interface {
	HasDemand(channelID string) bool
}

// GuildCache provides channel and role name lookups backed by Redis (or an in-memory map for tests).
// Keys are Discord Snowflake ID strings. The cache is populated on GUILD_CREATE and kept
// current via CHANNEL_*/GUILD_ROLE_* dispatch events.
type GuildCache interface {
	SetChannelName(ctx context.Context, channelID, name string) error
	GetChannelName(ctx context.Context, channelID string) (string, bool, error)
	DeleteChannelName(ctx context.Context, channelID string) error
	SetRoleName(ctx context.Context, roleID, name string) error
	GetRoleName(ctx context.Context, roleID string) (string, bool, error)
	DeleteRoleName(ctx context.Context, roleID string) error
}

// BuildIdentifyPayload constructs the op=2 IDENTIFY payload with the correct intents bitmask.
func BuildIdentifyPayload(token string) GatewayPayload {
	d, _ := json.Marshal(IdentifyData{
		Token:   token,
		Intents: RequiredIntents,
		Properties: IdentifyProperties{
			OS:      "linux",
			Browser: "allchat-discord-listener",
			Device:  "allchat-discord-listener",
		},
	})
	return GatewayPayload{Op: OpIdentify, D: json.RawMessage(d)}
}

// HandleReady processes the READY event: persists session_id and resume_gateway_url to Redis.
func HandleReady(ctx context.Context, data ReadyEventData, store SessionStore) error {
	if err := store.Set(ctx, RedisKeySessionID, data.SessionID); err != nil {
		return fmt.Errorf("failed to persist session_id: %w", err)
	}
	if err := store.Set(ctx, RedisKeyResumeURL, data.ResumeGatewayURL); err != nil {
		return fmt.Errorf("failed to persist resume_gateway_url: %w", err)
	}
	return nil
}

const (
	// staleLivenessThreshold is how long without any Gateway activity (heartbeat ACK or
	// any successful ReadMessage) before the liveness probe returns 503.
	// Discord sends HELLO with heartbeat_interval ~41s; 3 x 41s ≈ 2min, so 3 minutes
	// provides comfortable headroom while still catching zombie connections quickly.
	// A zero lastActivityAt (never connected) skips the check so the pod is not killed
	// before the initial session is established.
	staleLivenessThreshold = 3 * time.Minute
)

// GatewayClient manages the Discord Gateway WebSocket connection.
type GatewayClient struct {
	token            string
	gatewayURL       string
	store            SessionStore
	registry         ChannelRegistry
	publisher        MessagePublisher
	guildCache       GuildCache
	demandChecker    DemandChecker
	log              *zap.Logger
	conn             *websocket.Conn
	mu               sync.Mutex
	seq              int
	lastActivityAt   time.Time // Last time any Gateway message was received or heartbeat ACK was processed
	done             chan struct{}
	firstMessageSeen bool
	// OnReady is called in a goroutine after each successful READY event.
	// Use it to trigger post-connect checks (e.g. channel permission verification).
	OnReady func()
}

// NewGatewayClient creates a new GatewayClient.
// registry, publisher, and cache may be nil if the corresponding functionality is not needed
// (e.g. tests that only exercise heartbeat/READY handling), but all must be set for production use.
func NewGatewayClient(token, gatewayURL string, store SessionStore, log *zap.Logger, registry ChannelRegistry, pub MessagePublisher, cache GuildCache) *GatewayClient {
	return &GatewayClient{
		token:      token,
		gatewayURL: gatewayURL,
		store:      store,
		registry:   registry,
		publisher:  pub,
		guildCache: cache,
		log:        log,
		done:       make(chan struct{}),
	}
}

// SetDemandChecker wires a DemandChecker into the GatewayClient.
// When set, HandleMessageCreate drops messages for channels without active overlay demand.
func (c *GatewayClient) SetDemandChecker(dc DemandChecker) {
	c.demandChecker = dc
}

// Connect opens the Gateway WebSocket, runs HELLO/IDENTIFY/READY, starts heartbeat.
// This method blocks until the connection closes or ctx is cancelled.
// Each call to Connect resets the done channel so that a previous Close() call
// (e.g. from a leadership-lost callback) does not prevent reconnection.
func (c *GatewayClient) Connect(ctx context.Context) error {
	// Reset done channel so a previous Close() does not short-circuit this new session.
	c.mu.Lock()
	select {
	case <-c.done:
		// Channel was closed (by a prior Close() call) — create a fresh one.
		c.done = make(chan struct{})
	default:
		// Channel is still open — no reset needed.
	}
	c.mu.Unlock()

	// Use resume_gateway_url when a prior session exists — Discord requires RESUME to be sent
	// to the resume URL, not the standard gateway URL (sending RESUME to the wrong URL → 4002).
	connectURL := c.gatewayURL
	if resumeURL, err := c.store.Get(ctx, RedisKeyResumeURL); err == nil && resumeURL != "" {
		connectURL = resumeURL
	}

	c.log.Info("Connecting to Discord Gateway", zap.String("url", connectURL))

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, connectURL, http.Header{})
	if err != nil {
		return fmt.Errorf("failed to dial Gateway: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// connCtx is cancelled when this Connect() call exits, stopping the heartbeat goroutine
	// started below. Without this, old goroutines keep writing to c.conn after reconnect.
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			// Close code 4002 (invalid payload) or 4009 (session timeout) means the session is dead.
			// Clear Redis so the next Connect() falls back to a clean IDENTIFY on the standard URL.
			if ce, ok := err.(*websocket.CloseError); ok && (ce.Code == 4002 || ce.Code == 4009) {
				_ = c.store.Set(ctx, RedisKeySessionID, "")
				_ = c.store.Set(ctx, RedisKeyResumeURL, "")
				_ = c.store.Set(ctx, RedisKeySeq, "")
			}
			return fmt.Errorf("gateway read error: %w", err)
		}
		// Record activity on every successful read so the liveness probe detects
		// zombie connections (ReadMessage blocked with no data for >3 minutes).
		c.recordActivity()

		var payload GatewayPayload
		if err := json.Unmarshal(msg, &payload); err != nil {
			c.log.Warn("Failed to parse Gateway payload", zap.Error(err))
			continue
		}

		// Update sequence number
		if payload.S != nil {
			c.mu.Lock()
			c.seq = *payload.S
			c.mu.Unlock()
			_ = c.store.Set(ctx, RedisKeySeq, fmt.Sprintf("%d", *payload.S))
		}

		switch payload.Op {
		case OpHello:
			var hello HelloData
			if err := json.Unmarshal(payload.D, &hello); err != nil {
				return fmt.Errorf("failed to parse HELLO: %w", err)
			}
			// Start heartbeat goroutine with jitter on first beat (Discord requirement).
			// Pass connCtx so it stops when this Connect() call exits.
			go c.heartbeatLoop(connCtx, hello.HeartbeatInterval)

			// Attempt RESUME if a prior session exists in Redis; otherwise IDENTIFY.
			sessionID, _ := c.store.Get(ctx, RedisKeySessionID)
			resumeURL, _ := c.store.Get(ctx, RedisKeyResumeURL)
			seqStr, _ := c.store.Get(ctx, RedisKeySeq)
			seq, _ := strconv.Atoi(seqStr)

			if sessionID != "" && resumeURL != "" {
				resume := BuildResumePayload(c.token, sessionID, seq)
				if err := c.writeJSON(resume); err != nil {
					return fmt.Errorf("failed to send RESUME: %w", err)
				}
				metrics.IncResumeAttempt("success")
			} else {
				identify := BuildIdentifyPayload(c.token)
				if err := c.writeJSON(identify); err != nil {
					return fmt.Errorf("failed to send IDENTIFY: %w", err)
				}
				metrics.IncResumeAttempt("fallback_identify")
			}

		case OpDispatch:
			if payload.T != nil && *payload.T == "READY" {
				var ready ReadyEventData
				if err := json.Unmarshal(payload.D, &ready); err != nil {
					c.log.Warn("Failed to parse READY payload", zap.Error(err))
					continue
				}
				if err := HandleReady(ctx, ready, c.store); err != nil {
					c.log.Error("Failed to persist Gateway session", zap.Error(err))
				}
				c.log.Info("Gateway READY — session established",
					zap.String("session_id", ready.SessionID))
				if c.OnReady != nil {
					go c.OnReady()
				}
			}

			if payload.T != nil && *payload.T == "MESSAGE_CREATE" {
				var msg MessageCreateData
				if err := json.Unmarshal(payload.D, &msg); err != nil {
					c.log.Warn("Failed to parse MESSAGE_CREATE", zap.Error(err))
					continue
				}
				if err := c.HandleMessageCreate(ctx, msg); err != nil {
					return err
				}
			}

			if payload.T != nil && *payload.T == "MESSAGE_DELETE" {
			var del MessageDeleteData
			if err := json.Unmarshal(payload.D, &del); err != nil {
				c.log.Warn("Failed to parse MESSAGE_DELETE", zap.Error(err))
				continue
			}
			if err := c.HandleMessageDelete(ctx, del); err != nil {
				return err
			}
		}

		if payload.T != nil && *payload.T == "MESSAGE_DELETE_BULK" {
			var bulk MessageDeleteBulkData
			if err := json.Unmarshal(payload.D, &bulk); err != nil {
				c.log.Warn("Failed to parse MESSAGE_DELETE_BULK", zap.Error(err))
				continue
			}
			if err := c.HandleMessageDeleteBulk(ctx, bulk); err != nil {
				return err
			}
		}

		if payload.T != nil && *payload.T == "GUILD_CREATE" {
			var data GuildCreateData
			if err := json.Unmarshal(payload.D, &data); err != nil {
				c.log.Warn("Failed to parse GUILD_CREATE", zap.Error(err))
				continue
			}
			if err := c.HandleGuildCreate(ctx, data); err != nil {
				c.log.Warn("HandleGuildCreate failed", zap.Error(err))
			}
		}

		if payload.T != nil && (*payload.T == "CHANNEL_UPDATE" || *payload.T == "CHANNEL_CREATE") {
			var data ChannelUpdateData
			if err := json.Unmarshal(payload.D, &data); err != nil {
				c.log.Warn("Failed to parse CHANNEL_UPDATE/CREATE", zap.Error(err))
				continue
			}
			if err := c.HandleChannelUpdate(ctx, data); err != nil {
				c.log.Warn("HandleChannelUpdate failed", zap.Error(err))
			}
		}

		if payload.T != nil && *payload.T == "CHANNEL_DELETE" {
			var data ChannelUpdateData
			if err := json.Unmarshal(payload.D, &data); err != nil {
				c.log.Warn("Failed to parse CHANNEL_DELETE", zap.Error(err))
				continue
			}
			if err := c.HandleChannelDelete(ctx, data); err != nil {
				c.log.Warn("HandleChannelDelete failed", zap.Error(err))
			}
		}

		if payload.T != nil && (*payload.T == "GUILD_ROLE_UPDATE" || *payload.T == "GUILD_ROLE_CREATE") {
			var data GuildRoleUpdateData
			if err := json.Unmarshal(payload.D, &data); err != nil {
				c.log.Warn("Failed to parse GUILD_ROLE_UPDATE/CREATE", zap.Error(err))
				continue
			}
			if err := c.HandleGuildRoleUpdate(ctx, data); err != nil {
				c.log.Warn("HandleGuildRoleUpdate failed", zap.Error(err))
			}
		}

		if payload.T != nil && *payload.T == "GUILD_ROLE_DELETE" {
			var data GuildRoleDeleteData
			if err := json.Unmarshal(payload.D, &data); err != nil {
				c.log.Warn("Failed to parse GUILD_ROLE_DELETE", zap.Error(err))
				continue
			}
			if err := c.HandleGuildRoleDelete(ctx, data); err != nil {
				c.log.Warn("HandleGuildRoleDelete failed", zap.Error(err))
			}
		}

	case OpReconnect:
			c.log.Info("Gateway requested reconnect")
			return fmt.Errorf("gateway reconnect requested")

		case OpInvalidSession:
			c.log.Warn("Gateway invalidated session")
			// Parse d field — a boolean indicating whether the session is resumable.
			// d=false: must re-IDENTIFY; clear all session keys so the next Connect() does not RESUME.
			// d=true: may RESUME on next connect; preserve keys.
			var data InvalidSessionData
			if payload.D != nil {
				if parseErr := json.Unmarshal(payload.D, &data.Resumable); parseErr != nil {
					c.log.Warn("Failed to parse InvalidSession d field", zap.Error(parseErr))
				}
			}
			if !data.Resumable {
				_ = c.store.Set(ctx, RedisKeySessionID, "")
				_ = c.store.Set(ctx, RedisKeyResumeURL, "")
				_ = c.store.Set(ctx, RedisKeySeq, "")
			}
			return fmt.Errorf("gateway invalid session")

		case OpHeartbeatACK:
			c.log.Debug("Heartbeat ACK received")
			// Explicitly record activity on HeartbeatACK: this is the primary keep-alive
			// signal when no chat messages are flowing through configured channels.
			c.recordActivity()
		}
	}
}

// Close signals the connection loop to stop.
func (c *GatewayClient) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

// recordActivity updates the last-activity timestamp.
// Called on every successful conn.ReadMessage() and on every HeartbeatACK to ensure
// the liveness probe reflects actual Gateway keep-alive cadence, not just chat traffic.
func (c *GatewayClient) recordActivity() {
	c.mu.Lock()
	c.lastActivityAt = time.Now()
	c.mu.Unlock()
}

// LastActivityAt returns when the last Gateway message was received.
// Used by the health handler to surface idle duration in readiness responses.
func (c *GatewayClient) LastActivityAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastActivityAt
}

// IsStale returns true when the Gateway connection has gone silent for longer than
// staleLivenessThreshold.  A zero lastActivityAt (never connected) returns false
// so the pod is not killed before the initial session is established.
// Used by the liveness probe to trigger a Kubernetes pod restart when the Gateway
// connection is a zombie (ReadMessage blocks indefinitely with no data).
func (c *GatewayClient) IsStale() bool {
	c.mu.Lock()
	last := c.lastActivityAt
	c.mu.Unlock()
	if last.IsZero() {
		return false
	}
	return time.Since(last) > staleLivenessThreshold
}

// HandleMessageCreate processes a MESSAGE_CREATE dispatch event.
// It applies the following filters and enrichments before publishing:
//  1. Bot messages (author.bot == true) are silently dropped.
//  2. Messages from channels not in the registry are dropped at DEBUG.
//  3. The first MESSAGE_CREATE with empty content causes a service-halting error,
//     which indicates the MESSAGE_CONTENT privileged intent is not enabled.
//  4. Valid messages are enriched with tags and published to Redis Streams.
func (c *GatewayClient) HandleMessageCreate(ctx context.Context, msg MessageCreateData) error {
	// 1. Bot filter
	if msg.Author.Bot {
		if c.log != nil {
			c.log.Debug("Bot message filtered",
				zap.String("channel_id", msg.ChannelID),
				zap.String("author_id", msg.Author.ID),
			)
		}
		return nil
	}

	// 2. Channel registry lookup
	overlayID, found, err := c.registry.GetOverlayForChannel(ctx, msg.ChannelID)
	if err != nil {
		if c.log != nil {
			c.log.Warn("Channel registry lookup failed",
				zap.String("channel_id", msg.ChannelID),
				zap.Error(err),
			)
		}
		// Registry errors do not halt the service
		return nil
	}
	if !found {
		if c.log != nil {
			c.log.Debug("Channel not configured, dropping message",
				zap.String("channel_id", msg.ChannelID),
			)
		}
		return nil
	}

	// 2b. Demand check — drop messages for channels without active overlay demand
	if c.demandChecker != nil && !c.demandChecker.HasDemand(msg.ChannelID) {
		if c.log != nil {
			c.log.Debug("Channel has no overlay demand, dropping message",
				zap.String("channel_id", msg.ChannelID),
			)
		}
		return nil
	}

	// 3. Empty content check (first message only)
	c.mu.Lock()
	firstSeen := c.firstMessageSeen
	c.mu.Unlock()

	if !firstSeen {
		if msg.Content == "" {
			// Empty content on first MESSAGE_CREATE means MESSAGE_CONTENT privileged intent
			// is not enabled in the Discord Developer Portal. Log once and drop the message
			// but keep the service running — halting here causes a reconnect loop that silently
			// swallows every incoming message.
			if c.log != nil {
				c.log.Error("MESSAGE_CREATE with empty content — MESSAGE_CONTENT privileged intent is NOT enabled. "+
					"Enable it in Discord Developer Portal → Bot → Privileged Gateway Intents. "+
					"Messages will be dropped until this is fixed.",
					zap.String("channel_id", msg.ChannelID),
					zap.String("author_id", msg.Author.ID),
				)
			}
			c.mu.Lock()
			c.firstMessageSeen = true // suppress further per-message errors
			c.mu.Unlock()
			return nil
		}
		c.mu.Lock()
		c.firstMessageSeen = true
		c.mu.Unlock()
	}

	// 4. Build tags
	tags := map[string]string{
		"author_id":  msg.Author.ID,
		"avatar_url": discordAvatarURL(msg.Author.ID, msg.Author.Avatar, msg.GuildID, msg.Member),
	}
	if msg.Member != nil {
		if msg.Member.Nick != nil {
			tags["member_nick"] = *msg.Member.Nick
		}
		// role_color and badges require a role cache (populated on GUILD_CREATE).
		// If the cache is not populated, we leave these fields empty rather than
		// making a blocking REST call per message.
	}

	// 5. Resolve mentions in message text
	text := msg.Content
	if c.guildCache != nil {
		text = ResolveMentions(ctx, text, msg.Mentions, c.guildCache, c.log)
	}

	// 6. Parse timestamp
	ts, parseErr := time.Parse(time.RFC3339, msg.Timestamp)
	if parseErr != nil {
		ts = time.Now()
	}

	// 7. Build RawMessage (using interface{} to avoid circular import with publisher package)
	rawMsg := map[string]interface{}{
		"message_id":   msg.ID,
		"platform":     "discord",
		"overlay_id":   overlayID,
		"channel_id":   msg.ChannelID,
		"channel_name": msg.ChannelID,
		"user_id":      msg.Author.ID,
		"username":     msg.Author.Username,
		"text":         text,
		"tags":         tags,
		"timestamp":    ts,
	}

	// Record message received (passed all filters, about to publish)
	metrics.IncMessageReceived(msg.GuildID, msg.ChannelID)

	if err := c.publisher.Publish(ctx, rawMsg); err != nil {
		metrics.IncMessagePublished("error")
		return err
	}
	metrics.IncMessagePublished("success")
	return nil
}

// writeJSON sends a payload on the WebSocket connection, serialized as JSON.
func (c *GatewayClient) writeJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

// HandleMessageDelete processes a MESSAGE_DELETE dispatch event.
// It applies the channel filter (same registry as HandleMessageCreate) and
// publishes a deletion event to Redis Streams with EventType=message_deletion.
// Registry errors and unconfigured channels are silently dropped — the service continues.
func (c *GatewayClient) HandleMessageDelete(ctx context.Context, msg MessageDeleteData) error {
	overlayID, found, err := c.registry.GetOverlayForChannel(ctx, msg.ChannelID)
	if err != nil {
		if c.log != nil {
			c.log.Warn("Channel registry lookup failed on MESSAGE_DELETE",
				zap.String("channel_id", msg.ChannelID),
				zap.Error(err),
			)
		}
		return nil // registry errors do not halt service
	}
	if !found {
		return nil // not a configured channel
	}

	deleteEvent := map[string]interface{}{
		"message_id":   msg.ID + ":del",
		"platform":     "discord",
		"overlay_id":   overlayID,
		"channel_id":   msg.ChannelID,
		"channel_name": msg.ChannelID,
		"event_type":   "message_deletion",
		"event_data": map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": msg.ID,
		},
		"timestamp": time.Now(),
	}

	return c.publisher.Publish(ctx, deleteEvent)
}

// HandleMessageDeleteBulk processes a MESSAGE_DELETE_BULK dispatch event.
// It calls HandleMessageDelete once per ID in the IDs slice.
func (c *GatewayClient) HandleMessageDeleteBulk(ctx context.Context, bulk MessageDeleteBulkData) error {
	for _, id := range bulk.IDs {
		del := MessageDeleteData{
			ID:        id,
			ChannelID: bulk.ChannelID,
			GuildID:   bulk.GuildID,
		}
		if err := c.HandleMessageDelete(ctx, del); err != nil {
			return err
		}
	}
	return nil
}

// HandleGuildCreate processes a GUILD_CREATE dispatch event.
// It populates the GuildCache with channel and role names from the guild payload.
// Cache population is best-effort: errors are logged at WARN and processing continues.
func (c *GatewayClient) HandleGuildCreate(ctx context.Context, data GuildCreateData) error {
	if c.guildCache == nil {
		return nil
	}
	for _, ch := range data.Channels {
		if err := c.guildCache.SetChannelName(ctx, ch.ID, ch.Name); err != nil {
			if c.log != nil {
				c.log.Warn("Failed to cache channel name on GUILD_CREATE",
					zap.String("channel_id", ch.ID),
					zap.Error(err),
				)
			}
		}
	}
	for _, role := range data.Roles {
		if err := c.guildCache.SetRoleName(ctx, role.ID, role.Name); err != nil {
			if c.log != nil {
				c.log.Warn("Failed to cache role name on GUILD_CREATE",
					zap.String("role_id", role.ID),
					zap.Error(err),
				)
			}
		}
	}
	return nil
}

// HandleChannelUpdate processes CHANNEL_UPDATE and CHANNEL_CREATE dispatch events.
// It updates the channel name cache with the new name.
func (c *GatewayClient) HandleChannelUpdate(ctx context.Context, data ChannelUpdateData) error {
	if c.guildCache == nil {
		return nil
	}
	return c.guildCache.SetChannelName(ctx, data.ID, data.Name)
}

// HandleChannelDelete processes a CHANNEL_DELETE dispatch event.
// It removes the channel name from the cache.
func (c *GatewayClient) HandleChannelDelete(ctx context.Context, data ChannelUpdateData) error {
	if c.guildCache == nil {
		return nil
	}
	return c.guildCache.DeleteChannelName(ctx, data.ID)
}

// HandleGuildRoleUpdate processes GUILD_ROLE_UPDATE and GUILD_ROLE_CREATE dispatch events.
// It updates the role name cache.
func (c *GatewayClient) HandleGuildRoleUpdate(ctx context.Context, data GuildRoleUpdateData) error {
	if c.guildCache == nil {
		return nil
	}
	return c.guildCache.SetRoleName(ctx, data.Role.ID, data.Role.Name)
}

// HandleGuildRoleDelete processes a GUILD_ROLE_DELETE dispatch event.
// It removes the role from the cache.
func (c *GatewayClient) HandleGuildRoleDelete(ctx context.Context, data GuildRoleDeleteData) error {
	if c.guildCache == nil {
		return nil
	}
	return c.guildCache.DeleteRoleName(ctx, data.RoleID)
}

// discordAvatarURL returns the CDN URL for a Discord user's avatar.
// Priority: guild member avatar > user avatar > default avatar.
// Size 64 is sufficient for chat overlays.
func discordAvatarURL(userID string, userAvatar *string, guildID string, member *DiscordMember) string {
	const base = "https://cdn.discordapp.com"
	const size = "?size=64"

	// Guild-specific member avatar takes priority.
	if member != nil && member.Avatar != nil && *member.Avatar != "" {
		return base + "/guilds/" + guildID + "/users/" + userID + "/avatars/" + *member.Avatar + ".png" + size
	}

	// User avatar.
	if userAvatar != nil && *userAvatar != "" {
		return base + "/avatars/" + userID + "/" + *userAvatar + ".png" + size
	}

	// Default avatar — index derived from user ID per Discord docs.
	// For new-style usernames (no discriminator): (user_id >> 22) % 6.
	// We compute this from the string by parsing the snowflake.
	idx := defaultAvatarIndex(userID)
	return base + "/embed/avatars/" + idx + ".png"
}

// defaultAvatarIndex returns the default avatar index string for a user ID.
func defaultAvatarIndex(userID string) string {
	// Parse the snowflake as uint64, shift right 22 bits, mod 6.
	var id uint64
	for _, c := range userID {
		if c < '0' || c > '9' {
			return "0"
		}
		id = id*10 + uint64(c-'0')
	}
	return string(rune('0' + (id>>22)%6))
}

// reUserMention matches <@USER_ID> and <@!USER_ID> (guild member variant).
var reUserMention = regexp.MustCompile(`<@!?(\d+)>`)

// reChannelMention matches <#CHANNEL_ID>.
var reChannelMention = regexp.MustCompile(`<#(\d+)>`)

// reRoleMention matches <@&ROLE_ID>.
var reRoleMention = regexp.MustCompile(`<@&(\d+)>`)

// ResolveMentions replaces Discord mention tokens in text with human-readable names.
// mentions is the slice from MessageCreateData.Mentions (provides user name resolution).
// cache provides channel and role name lookups (may be nil — fallbacks used if nil).
// log may be nil (tests pass nil).
//
// Token resolution order: user mentions first, then channel, then role to avoid ambiguity.
func ResolveMentions(ctx context.Context, text string, mentions []DiscordUser, cache GuildCache, log *zap.Logger) string {
	// Build O(1) lookup map from the mentions array
	mentionMap := make(map[string]DiscordUser, len(mentions))
	for _, u := range mentions {
		mentionMap[u.ID] = u
	}

	// 1. Resolve user mentions: <@USER_ID> and <@!USER_ID>
	text = reUserMention.ReplaceAllStringFunc(text, func(match string) string {
		sub := reUserMention.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		id := sub[1]
		if u, ok := mentionMap[id]; ok {
			name := u.GlobalName
			if name == "" {
				name = u.Username
			}
			return "@" + name
		}
		if log != nil {
			log.Debug("Unresolvable user mention", zap.String("user_id", id))
		}
		return "@unknown"
	})

	// 2. Resolve channel mentions: <#CHANNEL_ID>
	text = reChannelMention.ReplaceAllStringFunc(text, func(match string) string {
		sub := reChannelMention.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		id := sub[1]
		if cache != nil {
			if name, found, err := cache.GetChannelName(ctx, id); err == nil && found {
				return "#" + name
			}
		}
		if log != nil {
			log.Debug("Unresolvable channel mention", zap.String("channel_id", id))
		}
		return "#channel"
	})

	// 3. Resolve role mentions: <@&ROLE_ID>
	text = reRoleMention.ReplaceAllStringFunc(text, func(match string) string {
		sub := reRoleMention.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		id := sub[1]
		if cache != nil {
			if name, found, err := cache.GetRoleName(ctx, id); err == nil && found {
				return "@" + name
			}
		}
		if log != nil {
			log.Debug("Unresolvable role mention", zap.String("role_id", id))
		}
		return "@unknown"
	})

	return text
}

// heartbeatLoop sends op=1 HEARTBEAT every interval ms.
// First heartbeat is delayed by interval * rand(0,1) per Discord requirements.
func (c *GatewayClient) heartbeatLoop(ctx context.Context, intervalMS int) {
	interval := time.Duration(intervalMS) * time.Millisecond
	jitter := time.Duration(float64(interval) * rand.Float64())
	select {
	case <-time.After(jitter):
	case <-ctx.Done():
		return
	case <-c.done:
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			seq := c.seq
			c.mu.Unlock()
			// Discord heartbeat: {"op":1,"d":SEQ} where d is the last sequence number or null.
			var d json.RawMessage
			if seq > 0 {
				d, _ = json.Marshal(seq)
			} else {
				d = json.RawMessage("null")
			}
			payload := GatewayPayload{Op: OpHeartbeat, D: d}
			if err := c.writeJSON(payload); err != nil {
				c.log.Warn("Heartbeat send failed", zap.Error(err))
				return
			}
			c.log.Debug("Heartbeat sent", zap.Int("seq", seq))
		case <-ctx.Done():
			return
		case <-c.done:
			return
		}
	}
}
