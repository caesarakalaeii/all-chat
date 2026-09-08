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

// Package websocket implements the Picarto chat WebSocket client.
//
// The endpoint (wss://chat.picarto.tv/chat/token=<jwt>) is undocumented; see
// docs/adr/0059-picarto-unofficial-popout-websocket.md for the spike evidence
// and the accepted risk. The parser is intentionally defensive: any frame that
// does not match a known shape is logged at Debug and dropped, never a panic.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/picarto-listener/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// chatURLFormat is the anonymous chat feed used by the pop-out chat page.
	// The JWT is generated per channel name (generateJwtToken GraphQL query,
	// resolved by the token fetcher in this package). Verified live during the
	// ADR-0059 spike.
	chatURLFormat = "wss://chat.picarto.tv/chat/token=%s"

	// Connection settings. Picarto's server sends WebSocket-level pings, so
	// gorilla's built-in pong handling applies: extend the read deadline in
	// the pong handler and read deadline reset.
	writeWait      = 10 * time.Second
	pongWait       = 120 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 512 * 1024 // 512 KB

	// controlFrameChat is the envelope discriminator for a chat batch.
	controlFrameChat = "c"

	// streamTypeFrame is the legacy "type":"stream" metadata frame.
	streamTypeFrame = "stream"

	// channelIDDigits keeps channel ids aware of their string form when the
	// server sends numbers.
)

// ConnectTokenFetcher produces a fresh chat JWT for a channel name.
type ConnectTokenFetcher func(ctx context.Context, channelName string) (string, error)

// MessageHandler is called for every parsed chat message.
type MessageHandler func(channelID string, message *ChatMessage)

// Client manages one WebSocket connection per subscribed channel.
type Client struct {
	logger         *zap.Logger
	tokenFetcher   ConnectTokenFetcher
	messageHandler MessageHandler

	mu     sync.Mutex
	conns  map[string]*channelConn // key: channel name (lowercase)
	ctx    context.Context
	cancel context.CancelFunc
}

// channelConn is a single live connection to a channel's chat feed.
type channelConn struct {
	conn         *websocket.Conn
	writeMu      sync.Mutex
	lastActivity time.Time
	closed       bool
}

// NewClient creates a Picarto WebSocket client.
func NewClient(logger *zap.Logger, tokenFetcher ConnectTokenFetcher, messageHandler MessageHandler) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		logger:         logger,
		tokenFetcher:   tokenFetcher,
		messageHandler: messageHandler,
		conns:          make(map[string]*channelConn),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect dials the chat feed for a channel. Returns an error when the token
// fetch or handshake fails; the read loop runs in the background.
func (c *Client) Connect(channelName string) error {
	key := strings.ToLower(channelName)

	c.mu.Lock()
	if existing, ok := c.conns[key]; ok && existing != nil && !existing.closed {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if c.tokenFetcher == nil {
		return fmt.Errorf("no token fetcher configured")
	}

	token, err := c.tokenFetcher(c.ctx, channelName)
	if err != nil {
		return fmt.Errorf("failed to fetch chat token for %s: %w", channelName, err)
	}

	url := fmt.Sprintf(chatURLFormat, token)
	c.logger.Info("Connecting to Picarto chat WebSocket",
		zap.String("channel", channelName),
	)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(c.ctx, url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Picarto chat: %w", err)
	}

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	cc := &channelConn{conn: conn, lastActivity: time.Now()}

	c.mu.Lock()
	if prev, ok := c.conns[key]; ok && prev != nil && !prev.closed {
		// Lost a race with a concurrent connect: close the extra dial.
		c.mu.Unlock()
		_ = conn.Close()
		return nil
	}
	c.conns[key] = cc
	c.mu.Unlock()

	metrics.SetSocketConnected(true)
	metrics.ObserveSubscription("subscribe")

	go c.readPump(key, channelName, cc)
	return nil
}

// Disconnect closes a channel's connection and forgets it.
func (c *Client) Disconnect(channelName string) {
	key := strings.ToLower(channelName)

	c.mu.Lock()
	cc, ok := c.conns[key]
	if ok {
		delete(c.conns, key)
	}
	c.mu.Unlock()

	if !ok {
		return
	}
	cc.closed = true
	cc.writeMu.Lock()
	_ = cc.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(writeWait))
	cc.writeMu.Unlock()
	_ = cc.conn.Close()

	metrics.ObserveSubscription("unsubscribe")
	c.maybeClearSocketGauge()
}

// DisconnectAll closes every channel connection (shutdown path).
func (c *Client) DisconnectAll() {
	c.mu.Lock()
	keys := make([]string, 0, len(c.conns))
	for key := range c.conns {
		keys = append(keys, key)
	}
	c.mu.Unlock()
	for _, key := range keys {
		c.Disconnect(key)
	}
}

// IsConnected reports whether the channel has a live connection.
func (c *Client) IsConnected(channelName string) bool {
	key := strings.ToLower(channelName)
	c.mu.Lock()
	defer c.mu.Unlock()
	cc, ok := c.conns[key]
	return ok && cc != nil && !cc.closed
}

// ActiveChannels returns the channel names with live connections.
func (c *Client) ActiveChannels() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, 0, len(c.conns))
	for key := range c.conns {
		names = append(names, key)
	}
	return names
}

// TokenFetcher exposes the configured fetcher (used by main for prewarming).
func (c *Client) TokenFetcher() ConnectTokenFetcher {
	return c.tokenFetcher
}

// MaybeReconnect reports whether a channel should be redialled; a channel
// whose connection dropped is not in the map anymore.
func (c *Client) MaybeReconnect(channelName string) bool {
	return !c.IsConnected(channelName)
}

func (c *Client) maybeClearSocketGauge() {
	c.mu.Lock()
	any := false
	for _, cc := range c.conns {
		if cc != nil && !cc.closed {
			any = true
			break
		}
	}
	c.mu.Unlock()
	if !any {
		metrics.SetSocketConnected(false)
	}
}

func (c *Client) readPump(key, channelName string, cc *channelConn) {
	defer func() {
		c.logger.Info("Picarto read loop stopped",
			zap.String("channel", channelName),
		)
		c.removeIfCurrent(key, cc)
		c.triggerReconnect(channelName)
	}()

	for {
		if cc.closed {
			return
		}
		_, message, err := cc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("Unexpected Picarto WebSocket close",
					zap.String("channel", channelName),
					zap.Error(err),
				)
			} else {
				c.logger.Info("Picarto WebSocket closed",
					zap.String("channel", channelName),
					zap.Error(err),
				)
			}
			metrics.IncReconnect("read_error")
			return
		}

		_ = cc.conn.SetReadDeadline(time.Now().Add(pongWait))
		cc.writeMu.Lock()
		cc.lastActivity = time.Now()
		cc.writeMu.Unlock()

		c.handleFrame(channelName, message)
	}
}

func (c *Client) removeIfCurrent(key string, cc *channelConn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.conns[key]; ok && cur == cc {
		delete(c.conns, key)
	}
}

// triggerReconnect records that a channel connection dropped and any leftover
// socket gauge is cleared. The dropped connection is already removed from the
// map; the manager's sync loop redials channels it still wants.
func (c *Client) triggerReconnect(channelName string) {
	metrics.IncReconnect("disconnected")
	metrics.SetSocketConnected(false)
}

// handleFrame parses one raw WebSocket payload. Unknown or malformed frames
// are logged at Debug and dropped — never panic (ADR-0059).
func (c *Client) handleFrame(channelName string, data []byte) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		c.logger.Debug("Dropping unparseable Picarto frame",
			zap.String("channel", channelName),
			zap.Int("size", len(data)),
		)
	}

	switch {
	case env.T == controlFrameChat:
		c.handleChatBatch(channelName, env.M)

	case env.Type == streamTypeFrame:
		// Stream metadata (viewers, multistream members). Chat-only listener:
		// logged for visibility, not published.
		var info StreamInfo
		payload := env.Messages
		if len(payload) == 0 {
			payload = env.M
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &info); err == nil && info.Name != "" {
				c.logger.Debug("Received stream metadata",
					zap.String("channel", channelName),
					zap.String("stream_name", info.Name),
				)
			}
		}

	case env.T == "" && env.Type == "":
		c.logger.Debug("Dropping empty Picarto envelope",
			zap.String("channel", channelName),
		)
		metrics.IncDropped("empty_envelope")

	default:
		// Unknown frame type: log-and-drop per ADR-0059.
		c.logger.Debug("Dropping unknown Picarto frame type",
			zap.String("channel", channelName),
			zap.String("t", env.T),
			zap.String("type", env.Type),
		)
		metrics.IncDropped("unknown_type")
	}
}

func (c *Client) handleChatBatch(channelName string, payload json.RawMessage) {
	if len(payload) == 0 {
		metrics.IncDropped("empty_batch")
		return
	}
	var batch []ChatEnvelope
	if err := json.Unmarshal(payload, &batch); err != nil {
		c.logger.Debug("Dropping malformed chat batch",
			zap.String("channel", channelName),
		)
		metrics.IncDropped("malformed_batch")
		return
	}

	for i := range batch {
		msg := &batch[i]
		if msg.Ct != controlFrameChat {
			c.logger.Debug("Dropping unknown item type in chat batch",
				zap.String("channel", channelName),
				zap.String("item_type", msg.Ct),
			)
			metrics.IncDropped("unknown_batch_item")
			continue
		}
		if c.messageHandler == nil {
			continue
		}
		chatMsg := &ChatMessage{
			ChannelID:   msg.ChannelID,
			UserID:      msg.UserID,
			Username:    msg.User,
			DisplayName: msg.DisplayName,
			AvatarURL:   avatarURL(msg.Avatar),
			Text:        msg.Message,
			MessageID:   msg.ID,
			Timestamp:   msg.D,
		}
		// Attribute the message to the source channel the overlay follows.
		// The server sets rn to the owning channel in multistreams; n stays
		// the actual chatter.
		c.messageHandler(channelName, chatMsg)
	}
}

// avatarURL expands a relative avatar path into the images CDN URL. Empty
// input stays empty; anything already absolute is passed through.
func avatarURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return "https://images.picarto.tv/" + path
}
