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
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	defaultHTTPTimeout = 30 * time.Second

	// staleLivenessThreshold is how long without any stream activity
	// (SSE keepalive comments or chat messages) before the liveness probe
	// returns 503. Rumble's keepalive cadence is short (seconds); 5 minutes
	// without any byte means the connection is zombie.
	staleLivenessThreshold = 5 * time.Minute
)

// Config carries the listener's connection settings.
type Config struct {
	// SessionCookie is an optional raw Cookie header value (e.g.
	// "session=<value>") forwarded on stream requests. Anonymous reads work
	// without it (spike, ADR-0061); supplying it preserves follower-scoped
	// detail that Rumble may filter for anonymous readers.
	SessionCookie string

	// BaseURL overrides the chat API base for tests. Empty in production.
	BaseURL string
}

// MessageHandler is called when a chat message is received on a chat stream.
type MessageHandler func(chatID string, message *SSEMessage)

// DeletionHandler is called when a message deletion is observed.
type DeletionHandler func(chatID string, messageID string)

// Client manages one SSE stream per subscribed Rumble chat room.
//
// Unlike kick-listener's single Pusher socket shared across chatrooms, Rumble's
// chat protocol has no multiplexing layer: each chat room is its own HTTP
// event stream. The Client therefore tracks one goroutine and one CancelFunc
// per chat room and exposes the same connection-lifecycle surface (Connect,
// Subscribe, Unsubscribe, IsConnected, ReconnectChan) the channel manager and
// health handler expect.
type Client struct {
	config          Config
	messageHandler  MessageHandler
	deletionHandler DeletionHandler
	log             *zap.Logger
	httpClient      *http.Client

	mu          sync.RWMutex
	streams     map[string]*chatStream // key: chat id
	connected   bool
	reconnectCh chan struct{}
}

// chatStream is the per-chat-room stream state.
type chatStream struct {
	chatID string
	cancel context.CancelFunc

	activeMu   sync.RWMutex
	lastActive time.Time
}

// NewClient creates an SSE stream client.
func NewClient(cfg Config, messageHandler MessageHandler, logger *zap.Logger) *Client {
	return &Client{
		config:         cfg,
		messageHandler: messageHandler,
		log:            logger,
		httpClient:     &http.Client{Timeout: defaultHTTPTimeout},
		streams:        make(map[string]*chatStream),
		reconnectCh:    make(chan struct{}, 16),
	}
}

// SetDeletionHandler sets the deletion callback.
func (c *Client) SetDeletionHandler(handler DeletionHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deletionHandler = handler
}

// Connect marks the client as ready to accept subscriptions. Rumble's SSE
// endpoint needs no handshake; per-stream connections are established by
// Subscribe. The method exists so cmd/main.go's startup ordering matches
// kick-listener's.
func (c *Client) Connect() error {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	c.log.Info("Rumble SSE client ready")
	return nil
}

// Subscribe opens the SSE stream for one chat room in a goroutine. The
// goroutine signals ReconnectChan whenever its stream ends uncleanly;
// cmd/main.go's reconnection loop drives the backoff and re-subscription.
func (c *Client) Subscribe(chatID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return errNotConnected{}
	}
	if _, exists := c.streams[chatID]; exists {
		c.log.Debug("stream already running", zap.String("chat_id", chatID))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream := &chatStream{chatID: chatID, cancel: cancel, lastActive: time.Now()}
	c.streams[chatID] = stream

	go c.runStream(ctx, stream)
	c.log.Info("subscribing to rumble chat stream", zap.String("chat_id", chatID))
	return nil
}

// Unsubscribe stops the stream for one chat room.
func (c *Client) Unsubscribe(chatID string) error {
	c.mu.Lock()
	stream, exists := c.streams[chatID]
	if exists {
		delete(c.streams, chatID)
	}
	c.mu.Unlock()

	if exists {
		stream.cancel()
	}
	return nil
}

// Disconnect stops every stream and marks the client offline.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	streams := c.streams
	c.streams = make(map[string]*chatStream)
	c.connected = false
	c.mu.Unlock()

	for _, stream := range streams {
		stream.cancel()
	}
	return nil
}

// IsConnected reports whether the client is in the ready state.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetSocketID exists for interface parity with kick-listener's status
// plumbing; SSE has no socket id, so it returns a constant.
func (c *Client) GetSocketID() string { return "sse" }

// ReconnectChan returns the channel signalled whenever a stream drops, so the
// caller can schedule a backoff reconnect.
func (c *Client) ReconnectChan() <-chan struct{} {
	return c.reconnectCh
}

// LastActivityAt returns the most recent stream activity across all streams,
// or the zero time when no stream has ever run.
func (c *Client) LastActivityAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var latest time.Time
	for _, stream := range c.streams {
		stream.activeMu.RLock()
		t := stream.lastActive
		stream.activeMu.RUnlock()
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

// IsStale reports whether every live stream has been silent beyond the stale
// threshold. A client with no streams is not stale (nothing has connected
// yet), mirroring kick-listener's behaviour.
func (c *Client) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.streams) == 0 {
		return false
	}
	for _, stream := range c.streams {
		stream.activeMu.RLock()
		last := stream.lastActive
		stream.activeMu.RUnlock()
		if time.Since(last) < staleLivenessThreshold {
			return false
		}
	}
	return true
}

// runStream pumps one chat room's SSE stream until it drops or the context is
// cancelled. On stream loss it notifies the reconnection loop — but only while
// the stream is still tracked, so Unsubscribe/Disconnect do not signal.
func (c *Client) runStream(ctx context.Context, stream *chatStream) {
	parser := NewStreamClient(loggerAdapter{c.log}, c.config.BaseURL)
	callbacks := StreamCallbacks{
		OnMessage: c.handleStreamMessage,
	}

	result := parser.Run(ctx, c.httpClient, stream.chatID, c.config.SessionCookie, callbacks)

	if result == streamConnLost {
		c.mu.RLock()
		_, stillTracked := c.streams[stream.chatID]
		c.mu.RUnlock()
		if stillTracked {
			select {
			case c.reconnectCh <- struct{}{}:
			default:
			}
		}
	}
}

// handleStreamMessage delivers a decoded message to the handler, applying the
// stream bookkeeping (activity stamp, deletion dispatch) in one place.
func (c *Client) handleStreamMessage(chatID string, msg SSEMessage) {
	c.mu.RLock()
	stream := c.streams[chatID]
	deletionHandler := c.deletionHandler
	messageHandler := c.messageHandler
	c.mu.RUnlock()

	if stream != nil {
		stream.activeMu.Lock()
		stream.lastActive = time.Now()
		stream.activeMu.Unlock()
	}

	if msg.IsDeleted {
		if deletionHandler != nil {
			deletionHandler(chatID, msg.ID)
		}
		return
	}
	if messageHandler != nil {
		messageHandler(chatID, &msg)
	}
}

// errNotConnected mirrors kick-listener's subscribe-before-connect failure.
type errNotConnected struct{}

func (errNotConnected) Error() string { return "SSE client not connected" }

// loggerAdapter adapts *zap.Logger to the parser's logger shim.
type loggerAdapter struct{ l *zap.Logger }

func (a loggerAdapter) Info(msg string, fields ...Field) {
	a.l.Info(msg, toZapFields(fields)...)
}

func (a loggerAdapter) Warn(msg string, fields ...Field) {
	a.l.Warn(msg, toZapFields(fields)...)
}

func toZapFields(fields []Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, zap.Any(f.Key, f.Value))
	}
	return out
}
