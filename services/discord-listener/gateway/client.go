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
	token      string
	gatewayURL string
	store      SessionStore
	log        *zap.Logger
	conn       *websocket.Conn
	mu         sync.Mutex
	seq        int
	done       chan struct{}
}

// NewGatewayClient creates a new GatewayClient.
func NewGatewayClient(token, gatewayURL string, store SessionStore, log *zap.Logger) *GatewayClient {
	return &GatewayClient{
		token:      token,
		gatewayURL: gatewayURL,
		store:      store,
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
				// Phase 27 startup assertion: MESSAGE_CONTENT is privileged and cannot be
				// confirmed from the READY payload alone. Log a warning so operators know
				// to verify it in the Discord Developer Portal. The hard check happens in
				// Phase 28 when MESSAGE_CREATE events are processed.
				// TODO(Phase 28): halt if first MESSAGE_CREATE has empty content
				c.log.Warn("MESSAGE_CONTENT is a privileged Gateway intent — " +
					"if Discord messages appear with empty content in overlays, " +
					"enable 'Message Content Intent' in Discord Developer Portal under Bot settings")
			}
			// Other dispatch events processed in later phases (Phase 28+)

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

// writeJSON sends a payload on the WebSocket connection, serialized as JSON.
func (c *GatewayClient) writeJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
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
