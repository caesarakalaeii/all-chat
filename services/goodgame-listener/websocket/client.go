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

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/goodgame-listener/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// defaultChatURL is the GoodGame chat WebSocket endpoint. Raw WebSocket,
	// not SockJS: verified live 2026-09 and confirmed by every maintained
	// third-party client (peka.online chat.go, p3ka chats/goodgame.go,
	// tommsawyer/goodgamechatapi lib/Connection.js) since the 2015 SockJS
	// era. The gist's http://chat.goodgame.ru:8081/chat SockJS URL is legacy.
	defaultChatURL = "wss://chat.goodgame.ru/chat/websocket"

	// Connection settings.
	writeWait  = 10 * time.Second
	readWait   = 90 * time.Second // reset on every frame; well above the 25s ping period
	pingPeriod = 25 * time.Second // application-level {"type":"ping"} frame
	// maxMessageSize bounds a single chat frame. Chat protocol documents 4KB
	// message bodies; motd/success_join/history payloads are slightly larger.
	maxMessageSize = 512 * 1024

	// staleLivenessThreshold mirrors the kick-listener liveness heuristic: a
	// connection that has seen nothing (frames OR our own pings failing) for
	// this long is considered zombie and the pod should be restarted.
	staleLivenessThreshold = 5 * time.Minute
)

// MessageHandler is called when a chat message is received for a channel.
// The string argument is the numeric channel id as a decimal string.
type MessageHandler func(channelID string, message *ChatMessageData)

// DeletionHandler is called when a message deletion event is received.
type DeletionHandler func(channelID string, event *RemoveMessageData)

// Client is a WebSocket client for the GoodGame chat server.
type Client struct {
	conn            *websocket.Conn
	connMu          sync.RWMutex
	writeMu         sync.Mutex // serialises writes to the WebSocket
	handlerMu       sync.RWMutex
	logger          *zap.Logger
	messageHandler  MessageHandler
	deletionHandler DeletionHandler
	url             string

	// Channel memberships keyed by numeric channel id string.
	subscribedChannels map[string]struct{}
	channelsMu         sync.RWMutex

	// Connection state.
	connected      bool
	welcomed       bool
	reconnectChan  chan struct{}
	lastActivityAt time.Time

	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new GoodGame chat WebSocket client.
func NewClient(messageHandler MessageHandler, logger *zap.Logger) *Client {
	return NewClientWithURL(defaultChatURL, messageHandler, logger)
}

// NewClientWithURL creates a client for an explicit endpoint (tests inject
// a httptest server URL here). Exported for that purpose only.
func NewClientWithURL(url string, messageHandler MessageHandler, logger *zap.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		logger:             logger,
		messageHandler:     messageHandler,
		url:                url,
		subscribedChannels: make(map[string]struct{}),
		reconnectChan:      make(chan struct{}, 1),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// SetDeletionHandler sets the deletion event handler.
func (c *Client) SetDeletionHandler(handler DeletionHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.deletionHandler = handler
}

// Connect dials the chat server. Retries once per call chain is left to the
// caller's reconnect loop; a single Connect either succeeds or fails fast.
func (c *Client) Connect() error {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}

	c.connMu.Lock()
	c.welcomed = false
	c.connMu.Unlock()

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to GoodGame chat: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connected = true
	c.connMu.Unlock()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(readWait))

	go c.readPump()
	go c.writePump()

	metrics.SetSocketConnected(true)
	c.logger.Info("Connected to GoodGame chat WebSocket", zap.String("url", c.url))
	return nil
}

// Disconnect closes the WebSocket connection.
func (c *Client) Disconnect() error {
	c.logger.Info("Disconnecting from GoodGame chat WebSocket")

	c.cancel()

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		c.writeMu.Lock()
		_ = c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		c.writeMu.Unlock()
		_ = c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	c.welcomed = false
	metrics.SetSocketConnected(false)
	return nil
}

// Subscribe sends a join frame for a numeric channel id. Like the
// kick-listener, the membership is recorded first; the frame is sent only
// once the welcome frame has been processed (isReady), and resubscribeAll
// replays the join on every (re)connect. The deferred case returns nil —
// delivery is guaranteed by the welcome replay.
func (c *Client) Subscribe(channelID int64) error {
	id := strconv.FormatInt(channelID, 10)

	c.channelsMu.Lock()
	if _, exists := c.subscribedChannels[id]; exists {
		c.channelsMu.Unlock()
		c.logger.Debug("Already joined channel", zap.String("channel_id", id))
		return nil
	}
	c.subscribedChannels[id] = struct{}{}
	c.channelsMu.Unlock()

	if !c.isReady() {
		c.logger.Debug("Connection not ready yet, join deferred to welcome replay",
			zap.String("channel_id", id))
		return nil
	}

	c.logger.Info("Joining GoodGame channel", zap.String("channel_id", id))
	return c.sendEnvelope(Envelope{
		Type: "join",
		Data: mustJSON(JoinRequest{ChannelID: id, Hidden: false}),
	})
}

// Unsubscribe sends an unjoin frame. When the connection is not ready the
// membership is simply dropped (nothing was joined on this connection).
func (c *Client) Unsubscribe(channelID int64) error {
	id := strconv.FormatInt(channelID, 10)

	c.channelsMu.Lock()
	if _, exists := c.subscribedChannels[id]; !exists {
		c.channelsMu.Unlock()
		return nil
	}
	delete(c.subscribedChannels, id)
	c.channelsMu.Unlock()

	if !c.isReady() {
		return nil
	}

	c.logger.Info("Unjoining GoodGame channel", zap.String("channel_id", id))
	return c.sendEnvelope(Envelope{
		Type: "unjoin",
		Data: mustJSON(UnjoinRequest{ChannelID: id}),
	})
}

// isReady reports a welcomed, live connection.
func (c *Client) isReady() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected && c.welcomed
}

// IsConnected reports the socket state.
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// GetSocketID returns "welcomed"/"" — the GoodGame protocol has no socket id,
// but channels.Manager records it for parity with the Kick flow.
func (c *Client) GetSocketID() string {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	if c.welcomed {
		return "welcomed"
	}
	return ""
}

// LastActivityAt returns when the last frame was received.
func (c *Client) LastActivityAt() time.Time {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.lastActivityAt
}

// IsStale reports a zombie connection (kick-listener parity).
func (c *Client) IsStale() bool {
	c.connMu.RLock()
	last := c.lastActivityAt
	c.connMu.RUnlock()
	if last.IsZero() {
		return false
	}
	return time.Since(last) > staleLivenessThreshold
}

// ReconnectChan exposes the reconnect signal consumed by cmd/main.go.
func (c *Client) ReconnectChan() <-chan struct{} {
	return c.reconnectChan
}

// resubscribeAll re-joins every recorded channel after a reconnect.
func (c *Client) resubscribeAll() {
	c.channelsMu.RLock()
	ids := make([]string, 0, len(c.subscribedChannels))
	for id := range c.subscribedChannels {
		ids = append(ids, id)
	}
	c.channelsMu.RUnlock()

	for _, id := range ids {
		if err := c.sendEnvelope(Envelope{
			Type: "join",
			Data: mustJSON(JoinRequest{ChannelID: id, Hidden: false}),
		}); err != nil {
			c.logger.Error("Failed to re-join channel after reconnect",
				zap.String("channel_id", id), zap.Error(err))
		}
	}
}

func (c *Client) sendEnvelope(env Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal %s frame: %w", env.Type, err)
	}

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to write %s frame: %w", env.Type, err)
	}
	return nil
}

func (c *Client) readPump() {
	defer func() {
		c.logger.Info("GoodGame read pump stopped")
		c.triggerReconnect("read loop stopped")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()
		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("GoodGame WebSocket read error", zap.Error(err))
			return
		}

		c.recordActivity()
		_ = conn.SetReadDeadline(time.Now().Add(readWait))
		c.handleFrame(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendEnvelope(Envelope{Type: "ping", Data: mustJSON(PingRequest{})}); err != nil {
				c.logger.Error("Failed to send GoodGame ping", zap.Error(err))
				return
			}
			c.logger.Debug("Sent GoodGame ping")
		}
	}
}

func (c *Client) handleFrame(data []byte) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		c.logger.Warn("Failed to unmarshal GoodGame frame", zap.ByteString("payload", truncate(data, 256)))
		return
	}

	switch env.Type {
	case "welcome":
		var w WelcomeData
		_ = json.Unmarshal(env.Data, &w)
		c.connMu.Lock()
		c.welcomed = true
		c.connMu.Unlock()
		c.logger.Info("GoodGame chat welcome",
			zap.Float64("protocol_version", w.ProtocolVersion),
			zap.String("server_ident", w.ServerIdent),
		)
		c.resubscribeAll()

	case "ping":
		// Server-initiated ping — answer with pong, protocol symmetric.
		var p PingRequest
		_ = json.Unmarshal(env.Data, &p)
		_ = c.sendEnvelope(Envelope{Type: "pong", Data: mustJSON(PongData{ChannelID: p.ChannelID})})

	case "pong":
		c.logger.Debug("Received GoodGame pong")

	case "success_join":
		var j SuccessJoinData
		if err := json.Unmarshal(env.Data, &j); err == nil {
			c.logger.Info("Joined GoodGame channel",
				zap.String("channel_id", j.ChannelID.String()),
				zap.String("channel_name", j.ChannelName),
				zap.String("clients_in_channel", j.ClientsInChannel.String()),
			)
		}
		metrics.ObserveSubscription("subscribe")

	case "success_unjoin":
		metrics.ObserveSubscription("unsubscribe")

	case "channel_counters":
		var cc ChannelCountersData
		if err := json.Unmarshal(env.Data, &cc); err == nil {
			metrics.SetChannelViewers(cc.ChannelID, mustInt64(cc.ClientsInChannel))
		}

	case "message":
		var msg ChatMessageData
		if err := json.Unmarshal(env.Data, &msg); err != nil {
			c.logger.Warn("Failed to unmarshal GoodGame chat message", zap.Error(err))
			metrics.IncDropped("unmarshal_error")
			return
		}
		id := msg.ChannelID.String()
		c.handlerMu.RLock()
		handler := c.messageHandler
		c.handlerMu.RUnlock()
		if handler != nil {
			handler(id, &msg)
		}

	case "remove_message":
		var rm RemoveMessageData
		if err := json.Unmarshal(env.Data, &rm); err != nil {
			c.logger.Warn("Failed to unmarshal GoodGame remove_message", zap.Error(err))
			return
		}
		c.handlerMu.RLock()
		handler := c.deletionHandler
		c.handlerMu.RUnlock()
		if handler != nil {
			handler(rm.ChannelID.String(), &rm)
		}

	case "error":
		var e ErrorData
		if err := json.Unmarshal(env.Data, &e); err == nil {
			c.logger.Warn("GoodGame chat error",
				zap.String("error_num", e.ErrorNum.String()),
				zap.String("msg", e.ErrorMsg),
				zap.String("channel_id", e.ChannelID.String()),
			)
		}
		metrics.IncDropped("protocol_error")

	default:
		// users_list, update_rights, payment, premium, motd, ... — chat
		// protocol carries many frame types this listener does not consume.
		c.logger.Debug("Unhandled GoodGame frame type", zap.String("type", env.Type))
	}
}

func (c *Client) recordActivity() {
	c.connMu.Lock()
	c.lastActivityAt = time.Now()
	c.connMu.Unlock()
}

// mustInt64 returns v as int64 (0 when unset).
func mustInt64(v json.Number) int64 {
	n, _ := v.Int64()
	return n
}

func (c *Client) triggerReconnect(reason string) {
	if c.ctx.Err() != nil {
		return
	}
	c.connMu.Lock()
	c.connected = false
	c.welcomed = false
	c.connMu.Unlock()

	c.logger.Warn("Scheduling GoodGame reconnect", zap.String("reason", reason))
	metrics.SetSocketConnected(false)
	metrics.IncReconnect(reason)

	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}

func mustJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		// All payloads here are plain structs; failure is a programming error.
		return json.RawMessage("{}")
	}
	return data
}

func truncate(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}
