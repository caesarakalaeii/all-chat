package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

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
	// Subscribe registers a channel to receive channelID strings on invalidation events.
	Subscribe(ctx context.Context, ch chan<- string) error
}

// MessagePublisher publishes a raw message to downstream consumers (e.g. Redis Streams).
// The message argument is typed as interface{} so the gateway package does not import
// the publisher package, avoiding a circular dependency.
type MessagePublisher interface {
	Publish(ctx context.Context, msg interface{}) error
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

// GatewayClient manages the Discord Gateway WebSocket connection.
type GatewayClient struct {
	token            string
	gatewayURL       string
	store            SessionStore
	registry         ChannelRegistry
	publisher        MessagePublisher
	log              *zap.Logger
	conn             *websocket.Conn
	mu               sync.Mutex
	seq              int
	done             chan struct{}
	firstMessageSeen bool
}

// NewGatewayClient creates a new GatewayClient.
// registry and publisher may be nil if MESSAGE_CREATE dispatch is not needed (e.g. tests
// that only exercise heartbeat/READY handling), but both must be set for production use.
func NewGatewayClient(token, gatewayURL string, store SessionStore, log *zap.Logger, registry ChannelRegistry, pub MessagePublisher) *GatewayClient {
	return &GatewayClient{
		token:      token,
		gatewayURL: gatewayURL,
		store:      store,
		registry:   registry,
		publisher:  pub,
		log:        log,
		done:       make(chan struct{}),
	}
}

// Connect opens the Gateway WebSocket, runs HELLO/IDENTIFY/READY, starts heartbeat.
// This method blocks until the connection closes or ctx is cancelled.
func (c *GatewayClient) Connect(ctx context.Context) error {
	c.log.Info("Connecting to Discord Gateway", zap.String("url", c.gatewayURL))

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, c.gatewayURL, http.Header{})
	if err != nil {
		return fmt.Errorf("failed to dial Gateway: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

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
			return fmt.Errorf("gateway read error: %w", err)
		}

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
			// Start heartbeat goroutine with jitter on first beat (Discord requirement)
			go c.heartbeatLoop(ctx, hello.HeartbeatInterval)

			// Send IDENTIFY
			identify := BuildIdentifyPayload(c.token)
			if err := c.writeJSON(identify); err != nil {
				return fmt.Errorf("failed to send IDENTIFY: %w", err)
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
				// MESSAGE_CONTENT is a privileged Gateway intent. Log a warning so operators
				// know to verify it in the Discord Developer Portal. The hard check happens
				// when the first MESSAGE_CREATE event is processed (empty content = intent missing).
				c.log.Warn("MESSAGE_CONTENT is a privileged Gateway intent — " +
					"if Discord messages appear with empty content in overlays, " +
					"enable 'Message Content Intent' in Discord Developer Portal under Bot settings")
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

	case OpReconnect:
			c.log.Info("Gateway requested reconnect")
			return fmt.Errorf("gateway reconnect requested")

		case OpInvalidSession:
			c.log.Warn("Gateway invalidated session")
			return fmt.Errorf("gateway invalid session")

		case OpHeartbeatACK:
			c.log.Debug("Heartbeat ACK received")
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

	// 3. Empty content check (first message only)
	c.mu.Lock()
	firstSeen := c.firstMessageSeen
	c.mu.Unlock()

	if !firstSeen && msg.Content == "" {
		if c.log != nil {
			c.log.Error("MESSAGE_CREATE with empty content — MESSAGE_CONTENT privileged intent not enabled; halting",
				zap.String("channel_id", msg.ChannelID),
			)
		}
		return fmt.Errorf("missing MESSAGE_CONTENT intent: first MESSAGE_CREATE had empty content")
	}

	c.mu.Lock()
	c.firstMessageSeen = true
	c.mu.Unlock()

	// 4. Build tags
	tags := map[string]string{
		"author_id": msg.Author.ID,
	}
	if msg.Member != nil {
		if msg.Member.Nick != nil {
			tags["member_nick"] = *msg.Member.Nick
		}
		// role_color and badges require a role cache (populated on GUILD_CREATE).
		// If the cache is not populated, we leave these fields empty rather than
		// making a blocking REST call per message.
	}

	// 5. Parse timestamp
	ts, parseErr := time.Parse(time.RFC3339, msg.Timestamp)
	if parseErr != nil {
		ts = time.Now()
	}

	// 6. Build RawMessage (using interface{} to avoid circular import with publisher package)
	rawMsg := map[string]interface{}{
		"message_id":   msg.ID,
		"platform":     "discord",
		"overlay_id":   overlayID,
		"channel_id":   msg.ChannelID,
		"channel_name": msg.ChannelID,
		"user_id":      msg.Author.ID,
		"username":     msg.Author.Username,
		"text":         msg.Content,
		"tags":         tags,
		"timestamp":    ts,
	}

	return c.publisher.Publish(ctx, rawMsg)
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
			var seqField *int
			if seq > 0 {
				s := seq
				seqField = &s
			}
			payload := GatewayPayload{Op: OpHeartbeat, S: seqField}
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
