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

// Package websocket holds one Owncast chat WebSocket client per instance.
// Unlike Kick (one WS with many Pusher subscriptions), an Owncast instance
// serves ONE stream, so the unit of connection is the instance URL itself.
package websocket

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/owncast-listener/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// pongWait mirrors Owncast's server-side pongWait (60s in
	// services/chat/chatclient.go); the client must keep reading pings and
	// respond well inside it. 90s gives slack above the server's 60s ping
	// period (which is its pongWait*9/10).
	pongWait   = 90 * time.Second
	pingPeriod = (pongWait * 9) / 10

	maxMessageSize = 512 * 1024
)

// MessageHandler is called for every CHAT event received from an instance.
// instanceURL is the normalized Owncast base URL (the "channel").
type MessageHandler func(instanceURL string, msg *OwncastChatMessage)

// OwncastChatMessage is a chat message broadcast payload from Owncast's
// websocket (/ws). Shape grounded in owncast/owncast develop:
// services/chat/events/userMessageEvent.go GetBroadcastPayload (id, timestamp,
// body, user, type, visible) and models/user.go (user.id, displayName,
// displayColor, isBot, authenticated).
type OwncastChatMessage struct {
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"` // RFC3339 time.Time rendered by Owncast
	Type      string          `json:"type"`      // "CHAT", "SYSTEM", "CHAT_ACTION", ...
	Body      string          `json:"body"`
	Visible   bool            `json:"visible"`
	User      OwncastChatUser `json:"user"`
}

// OwncastChatUser is the user object embedded in chat events (owncast
// models.User serialized to chat clients).
type OwncastChatUser struct {
	ID            string     `json:"id"`
	DisplayName   string     `json:"displayName"`
	DisplayColor  int        `json:"displayColor"`
	IsBot         bool       `json:"isBot"`
	Authenticated bool       `json:"authenticated"`
	CreatedAt     *time.Time `json:"createdAt,omitempty"`
	PreviousNames []string   `json:"previousNames,omitempty"`
}

// Client is one websocket connection to one Owncast instance. It is owned by
// the channel manager: Connect/Disconnect are serialized by the manager
// (one runLoop goroutine per source).
type Client struct {
	instanceURL string // normalized base URL, e.g. https://watch.example.org
	wsURL       string // derived ws(s)://.../ws?accessToken=...
	token       string // from /api/chat/register
	logger      *zap.Logger

	conn *websocket.Conn
	done chan struct{}

	lastActivityAt time.Time
	activityMu     sync.Mutex

	handler MessageHandler
}

// NewClient builds a client for a normalized instance URL.
func NewClient(instanceURL, token string, handler MessageHandler, logger *zap.Logger) *Client {
	log := logger
	if log == nil {
		log = zap.NewNop()
	}
	return &Client{
		instanceURL: instanceURL,
		wsURL:       buildWSURL(instanceURL, token),
		token:       token,
		logger:      log,
		done:        make(chan struct{}),
		handler:     handler,
	}
}

// buildWSURL derives the chat websocket URL and auth query parameter from the
// instance base URL and a registered access token. Owncast serves the socket
// at /ws and requires accessToken as a query param (services/chat/server.go
// reads r.URL.Query().Get("accessToken")).
func buildWSURL(instanceURL, token string) string {
	wsScheme := "ws"
	if strings.HasPrefix(instanceURL, "https://") {
		wsScheme = "wss"
	} else {
		wsScheme = "ws"
	}
	return wsScheme + "://" + strings.TrimPrefix(strings.TrimPrefix(instanceURL, "https://"), "http://") + "/ws?accessToken=" + token
}

// Connect registers (re-registers) and dials the socket. Registration is
// repeated on every connect attempt: tokens are per-session server side and
// an instance restart invalidates them.
func (c *Client) Connect(token string) error {
	c.token = token
	c.wsURL = buildWSURL(c.instanceURL, token)

	conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	c.conn.SetReadLimit(maxMessageSize)
	c.setActivity(time.Now())
	return nil
}

// Disconnect closes the socket and stops the read loop.
func (c *Client) Disconnect() error {
	close(c.done)
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected reports socket presence.
func (c *Client) IsConnected() bool {
	return c.conn != nil
}

// Run reads frames until the socket dies or Disconnect is called. Frames from
// Owncast can carry multiple newline-separated JSON events
// (chatclient.go writePump coalesces queued events with '\n').
func (c *Client) Run() {
	var conn *websocket.Conn
	var pongOnce sync.Once

	for {
		select {
		case <-c.done:
			return
		default:
		}

		if c.conn == nil {
			return
		}
		conn = c.conn

		conn.SetReadDeadline(time.Now().Add(pongWait))
		// Apply the pong handler once per connection.
		pongOnce.Do(func() {
			conn.SetPongHandler(func(string) error {
				c.setActivity(time.Now())
				return conn.SetReadDeadline(time.Now().Add(pongWait))
			})
		})

		_, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-c.done:
			default:
				c.logger.Debug("owncast socket read ended",
					zap.String("instance", c.instanceURL),
					zap.Error(err),
				)
			}
			metrics.SetSocketConnected(c.instanceURL, false)
			return
		}
		c.setActivity(time.Now())

		// Server may batch several events into one frame.
		c.handleEvent(string(data))
	}
}

// handleEvent parses one frame (which may carry several newline-separated
// JSON events — Owncast coalesces queued events, chatclient.go writePump)
// and forwards CHAT messages to the handler. Anything else (system messages,
// user join/part, name changes, stream start/stop, actions) is
// logged-and-dropped: chat only, per the plan.
func (c *Client) handleEvent(frame string) {
	for _, line := range strings.Split(frame, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		c.handleSingleEvent(line)
	}
}

// handleSingleEvent decodes one event line.
func (c *Client) handleSingleEvent(line string) {
	var env OwncastChatMessage
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		c.logger.Debug("owncast: undecodable event dropped",
			zap.String("instance", c.instanceURL),
			zap.Error(err),
		)
		return
	}
	if env.Type != "CHAT" {
		// Join/part, system messages, moderation visibility, actions etc.
		// are deliberately ignored (chat only).
		return
	}
	if env.ID == "" && env.Body == "" {
		return
	}
	c.handler(c.instanceURL, &env)
}

// LastActivityAt returns when the client last read anything from the socket.
func (c *Client) LastActivityAt() time.Time {
	c.activityMu.Lock()
	defer c.activityMu.Unlock()
	return c.lastActivityAt
}

// IsStale reports no socket activity beyond staleThreshold.
func (c *Client) IsStale() bool {
	c.activityMu.Lock()
	last := c.lastActivityAt
	c.activityMu.Unlock()
	if last.IsZero() {
		return false
	}
	return time.Since(last) > staleThreshold
}

func (c *Client) setActivity(t time.Time) {
	c.activityMu.Lock()
	c.lastActivityAt = t
	c.activityMu.Unlock()
}

const staleThreshold = 5 * time.Minute
