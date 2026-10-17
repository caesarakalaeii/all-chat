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
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/metrics"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Kick's Centrifugo refuses the 129th subscription on one connection
	// (error 106 "limit exceeded", measured 2026-10-18); stay well below it.
	centrifugoMaxPerConn = 100
	centrifugoMaxBackoff = 60 * time.Second
	// Extra slack on top of the server's announced ping interval before a
	// silent connection counts as dead.
	centrifugoPingSlack = 15 * time.Second
	// Centrifugo error 105: already subscribed. Harmless when a resubscribe
	// after reconnect races a fresh Subscribe.
	centrifugoAlreadySubscribed = 105
)

// CentrifugoClient reads Kick chat from Kick's Centrifugo server (ADR-0077).
// Chatrooms are spread over a pool of connections because one connection may
// hold at most 128 subscriptions. Each connection fetches its own guest
// token, refreshes it before it expires, answers pings, and reconnects with
// backoff on its own, resubscribing its chatrooms.
type CentrifugoClient struct {
	url      string
	tokenURL string
	logger   *zap.Logger

	onMessage MessageHandler
	handlerMu sync.RWMutex
	onDelete  DeletionHandler

	maxPerConn int
	minBackoff time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.Mutex
	conns     []*centrifugoConn // conns[0] is never closed while the client runs
	byChannel map[string]*centrifugoConn
}

// NewCentrifugoClient creates a client for the Centrifugo websocket URL Kick
// handed out, taking guest tokens from tokenURL.
func NewCentrifugoClient(url, tokenURL string, onMessage MessageHandler, logger *zap.Logger) *CentrifugoClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &CentrifugoClient{
		url:        url,
		tokenURL:   tokenURL,
		logger:     logger,
		onMessage:  onMessage,
		maxPerConn: centrifugoMaxPerConn,
		minBackoff: time.Second,
		ctx:        ctx,
		cancel:     cancel,
		byChannel:  make(map[string]*centrifugoConn),
	}
}

// SetDeletionHandler sets the deletion event handler.
func (c *CentrifugoClient) SetDeletionHandler(handler DeletionHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onDelete = handler
}

// Connect opens the first connection and waits for its handshake. Further
// connections open as Subscribe needs room.
func (c *CentrifugoClient) Connect() error {
	c.mu.Lock()
	if len(c.conns) > 0 {
		c.mu.Unlock()
		return nil
	}
	conn := c.newConnLocked()
	c.mu.Unlock()

	first := make(chan error, 1)
	go conn.run(first)
	if err := <-first; err != nil {
		c.mu.Lock()
		c.conns = nil
		c.mu.Unlock()
		conn.stop()
		return fmt.Errorf("connect to Kick Centrifugo: %w", err)
	}
	return nil
}

// Disconnect closes every connection.
func (c *CentrifugoClient) Disconnect() error {
	c.cancel()
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.mu.Unlock()
	for _, conn := range conns {
		conn.stop()
	}
	metrics.SetSocketConnected(false)
	return nil
}

// Subscribe adds a chatroom to a connection with room, opening a new
// connection when every existing one is full.
func (c *CentrifugoClient) Subscribe(chatroomID int) error {
	channel := fmt.Sprintf(kickChannelFormat, chatroomID)

	c.mu.Lock()
	if _, ok := c.byChannel[channel]; ok {
		c.mu.Unlock()
		return nil
	}
	var conn *centrifugoConn
	for _, k := range c.conns {
		if k.channelCount() < c.maxPerConn {
			conn = k
			break
		}
	}
	started := conn != nil
	if !started {
		conn = c.newConnLocked()
	}
	conn.addChannel(channel)
	c.byChannel[channel] = conn
	c.mu.Unlock()

	if !started {
		// Its first session subscribes everything in its channel set.
		go conn.run(nil)
		return nil
	}
	conn.subscribe(channel)
	return nil
}

// SubscribeWithAuth subscribes like Subscribe: Kick chatrooms are public
// Centrifugo channels, and the token is a Pusher channel signature.
func (c *CentrifugoClient) SubscribeWithAuth(chatroomID int, _ string) error {
	return c.Subscribe(chatroomID)
}

// Unsubscribe drops a chatroom, and closes its connection if that left it
// empty (except the first one).
func (c *CentrifugoClient) Unsubscribe(chatroomID int) error {
	channel := fmt.Sprintf(kickChannelFormat, chatroomID)

	c.mu.Lock()
	conn, ok := c.byChannel[channel]
	if !ok {
		c.mu.Unlock()
		return nil
	}
	delete(c.byChannel, channel)
	empty := conn.removeChannel(channel) == 0
	closeConn := empty && len(c.conns) > 0 && conn != c.conns[0]
	if closeConn {
		for i, k := range c.conns {
			if k == conn {
				c.conns = append(c.conns[:i], c.conns[i+1:]...)
				break
			}
		}
	}
	c.mu.Unlock()

	if closeConn {
		conn.stop()
		c.updateState()
		return nil
	}
	if conn.isConnected() {
		_ = conn.send("unsubscribe", map[string]string{"channel": channel})
	}
	return nil
}

// IsConnected reports whether every connection in the pool is up.
func (c *CentrifugoClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.conns) == 0 {
		return false
	}
	for _, k := range c.conns {
		if !k.isConnected() {
			return false
		}
	}
	return true
}

// GetSocketID returns "": the Pusher channel auth that needs a socket ID has
// no Centrifugo counterpart, so callers fall back to Subscribe.
func (c *CentrifugoClient) GetSocketID() string { return "" }

// LastActivityAt returns the activity time of the quietest connection that
// has ever been active, so one dead connection is enough to go stale.
func (c *CentrifugoClient) LastActivityAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	var oldest time.Time
	for _, k := range c.conns {
		if t := k.lastActivity(); !t.IsZero() && (oldest.IsZero() || t.Before(oldest)) {
			oldest = t
		}
	}
	return oldest
}

// IsStale reports a connection silent for longer than the liveness threshold.
func (c *CentrifugoClient) IsStale() bool {
	last := c.LastActivityAt()
	return !last.IsZero() && time.Since(last) > staleLivenessThreshold
}

// ReconnectChan never fires: each connection reconnects itself.
func (c *CentrifugoClient) ReconnectChan() <-chan struct{} { return nil }

func (c *CentrifugoClient) newConnLocked() *centrifugoConn {
	ctx, cancel := context.WithCancel(c.ctx)
	conn := &centrifugoConn{
		c:        c,
		n:        len(c.conns),
		ctx:      ctx,
		cancel:   cancel,
		clientID: uuid.NewString(),
		channels: make(map[string]struct{}),
		pending:  make(map[uint32]string),
	}
	c.conns = append(c.conns, conn)
	return conn
}

func (c *CentrifugoClient) updateState() {
	metrics.SetSocketConnected(c.IsConnected())
}

func (c *CentrifugoClient) dispatch(channel, event string, data json.RawMessage) {
	c.handlerMu.RLock()
	onDelete := c.onDelete
	c.handlerMu.RUnlock()
	dispatchKickEvent(c.logger, event, channel, data, c.onMessage, onDelete)
}

// centrifugoConn is one websocket in the pool.
type centrifugoConn struct {
	c        *CentrifugoClient
	n        int
	ctx      context.Context
	cancel   context.CancelFunc
	clientID string

	mu         sync.Mutex
	ws         *websocket.Conn
	connected  bool
	channels   map[string]struct{}
	pending    map[uint32]string // subscribe command id -> channel, for error logs
	activityAt time.Time
	refresh    *time.Timer

	writeMu sync.Mutex
	nextID  atomic.Uint32
}

type centrifugoError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type centrifugoReply struct {
	ID      uint32            `json:"id"`
	Error   *centrifugoError  `json:"error"`
	Connect *centrifugoExpiry `json:"connect"`
	Refresh *centrifugoExpiry `json:"refresh"`
	Push    *struct {
		Channel string `json:"channel"`
		Pub     *struct {
			Data struct {
				Event string          `json:"event"`
				Data  json.RawMessage `json:"data"`
			} `json:"data"`
		} `json:"pub"`
		Disconnect  *centrifugoError `json:"disconnect"`
		Unsubscribe *centrifugoError `json:"unsubscribe"`
	} `json:"push"`
}

type centrifugoExpiry struct {
	Expires bool `json:"expires"`
	TTL     int  `json:"ttl"`
	Ping    int  `json:"ping"`
}

// run keeps the connection up until it is stopped. first, when set, receives
// the outcome of the first handshake.
func (k *centrifugoConn) run(first chan<- error) {
	report := func(err error) {
		if first != nil {
			first <- err
			first = nil
		}
	}
	backoff := k.c.minBackoff
	for {
		started := time.Now()
		err := k.session(func() { report(nil) })
		k.markDown()
		if k.ctx.Err() != nil {
			report(k.ctx.Err())
			return
		}
		if first != nil {
			report(err)
			return
		}
		if time.Since(started) > time.Minute {
			backoff = k.c.minBackoff
		}
		k.c.logger.Warn("Kick Centrifugo connection lost, reconnecting",
			zap.Int("conn", k.n),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		metrics.IncReconnect("centrifugo session ended")
		select {
		case <-k.ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, centrifugoMaxBackoff)
	}
}

// session connects, subscribes this connection's chatrooms and reads until
// the connection fails.
func (k *centrifugoConn) session(onConnected func()) error {
	token, err := fetchCentrifugoToken(k.ctx, k.c.tokenURL, k.clientID)
	if err != nil {
		return fmt.Errorf("fetch token: %w", err)
	}

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	ws, _, err := dialer.DialContext(k.ctx, k.c.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer ws.Close()
	ws.SetReadLimit(maxMessageSize)

	k.mu.Lock()
	k.ws = ws
	k.mu.Unlock()
	if k.ctx.Err() != nil { // stopped while dialing
		return k.ctx.Err()
	}

	if err := k.send("connect", map[string]string{"token": token, "name": "js"}); err != nil {
		return err
	}
	_ = ws.SetReadDeadline(time.Now().Add(writeWait))
	_, frame, err := ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("read connect reply: %w", err)
	}
	var reply centrifugoReply
	if err := json.Unmarshal([]byte(strings.SplitN(string(frame), "\n", 2)[0]), &reply); err != nil {
		return fmt.Errorf("decode connect reply: %w", err)
	}
	if reply.Error != nil {
		return fmt.Errorf("connect refused: %d %s", reply.Error.Code, reply.Error.Message)
	}
	if reply.Connect == nil {
		return fmt.Errorf("unexpected connect reply: %s", frame)
	}

	readWait := pongWait
	if reply.Connect.Ping > 0 {
		readWait = time.Duration(reply.Connect.Ping)*time.Second + centrifugoPingSlack
	}

	k.mu.Lock()
	k.connected = true
	k.activityAt = time.Now()
	channels := make([]string, 0, len(k.channels))
	for ch := range k.channels {
		channels = append(channels, ch)
	}
	k.mu.Unlock()
	k.scheduleRefresh(reply.Connect)
	k.c.updateState()
	k.c.logger.Info("Connected to Kick Centrifugo",
		zap.Int("conn", k.n),
		zap.Int("channels", len(channels)),
	)
	onConnected()

	for _, ch := range channels {
		k.subscribe(ch)
	}

	for {
		_ = ws.SetReadDeadline(time.Now().Add(readWait))
		_, frame, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		k.mu.Lock()
		k.activityAt = time.Now()
		k.mu.Unlock()
		for _, line := range strings.Split(string(frame), "\n") {
			if err := k.handleLine(line); err != nil {
				return err
			}
		}
	}
}

func (k *centrifugoConn) handleLine(line string) error {
	line = strings.TrimSpace(line)
	switch line {
	case "":
		return nil
	case "{}": // server ping
		return k.write([]byte("{}"))
	}

	var r centrifugoReply
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		k.c.logger.Warn("Undecodable Kick Centrifugo frame", zap.Int("conn", k.n), zap.Error(err))
		return nil
	}

	k.mu.Lock()
	channel := k.pending[r.ID]
	delete(k.pending, r.ID)
	k.mu.Unlock()

	switch {
	case r.Push != nil && r.Push.Pub != nil:
		k.c.dispatch(r.Push.Channel, r.Push.Pub.Data.Event, r.Push.Pub.Data.Data)
	case r.Push != nil && r.Push.Disconnect != nil:
		return fmt.Errorf("server disconnect: %d %s", r.Push.Disconnect.Code, r.Push.Disconnect.Message)
	case r.Push != nil && r.Push.Unsubscribe != nil:
		k.c.logger.Warn("Kick Centrifugo unsubscribed a channel",
			zap.Int("conn", k.n),
			zap.String("channel", r.Push.Channel),
			zap.Int("code", r.Push.Unsubscribe.Code),
		)
	case r.Refresh != nil:
		k.scheduleRefresh(r.Refresh)
	case r.Error != nil && r.Error.Code == centrifugoAlreadySubscribed:
	case r.Error != nil:
		k.c.logger.Error("Kick Centrifugo command failed",
			zap.Int("conn", k.n),
			zap.String("channel", channel),
			zap.Int("code", r.Error.Code),
			zap.String("message", r.Error.Message),
		)
	}
	return nil
}

// scheduleRefresh renews the connection token at three quarters of its ttl.
// A failed refresh is only logged: the server then disconnects on expiry and
// the reconnect fetches a fresh token.
func (k *centrifugoConn) scheduleRefresh(e *centrifugoExpiry) {
	if !e.Expires || e.TTL <= 0 {
		return
	}
	delay := time.Duration(e.TTL) * time.Second * 3 / 4
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.refresh != nil {
		k.refresh.Stop()
	}
	k.refresh = time.AfterFunc(delay, func() {
		token, err := fetchCentrifugoToken(k.ctx, k.c.tokenURL, k.clientID)
		if err == nil {
			err = k.send("refresh", map[string]string{"token": token})
		}
		if err != nil && k.ctx.Err() == nil {
			k.c.logger.Warn("Kick Centrifugo token refresh failed", zap.Int("conn", k.n), zap.Error(err))
		}
	})
}

// subscribe sends a subscribe once the connect handshake is done; before
// that, or after a drop, the next session subscribes the channel instead.
func (k *centrifugoConn) subscribe(channel string) {
	if !k.isConnected() {
		return
	}
	id, err := k.sendID("subscribe", map[string]string{"channel": channel})
	if err != nil {
		return
	}
	k.mu.Lock()
	k.pending[id] = channel
	k.mu.Unlock()
}

func (k *centrifugoConn) send(kind string, payload any) error {
	_, err := k.sendID(kind, payload)
	return err
}

func (k *centrifugoConn) sendID(kind string, payload any) (uint32, error) {
	id := k.nextID.Add(1)
	b, err := json.Marshal(map[string]any{"id": id, kind: payload})
	if err != nil {
		return 0, err
	}
	return id, k.write(b)
}

func (k *centrifugoConn) write(b []byte) error {
	k.mu.Lock()
	ws := k.ws
	k.mu.Unlock()
	if ws == nil {
		return errors.New("not connected")
	}
	k.writeMu.Lock()
	defer k.writeMu.Unlock()
	_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
	return ws.WriteMessage(websocket.TextMessage, b)
}

func (k *centrifugoConn) markDown() {
	k.mu.Lock()
	k.connected = false
	k.ws = nil
	if k.refresh != nil {
		k.refresh.Stop()
		k.refresh = nil
	}
	clear(k.pending)
	k.mu.Unlock()
	k.c.updateState()
}

func (k *centrifugoConn) stop() {
	k.cancel()
	k.mu.Lock()
	ws := k.ws
	k.mu.Unlock()
	if ws != nil {
		k.writeMu.Lock()
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		k.writeMu.Unlock()
		_ = ws.Close()
	}
}

func (k *centrifugoConn) isConnected() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.connected
}

func (k *centrifugoConn) lastActivity() time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.activityAt
}

func (k *centrifugoConn) channelCount() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.channels)
}

func (k *centrifugoConn) addChannel(channel string) {
	k.mu.Lock()
	k.channels[channel] = struct{}{}
	k.mu.Unlock()
}

func (k *centrifugoConn) removeChannel(channel string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.channels, channel)
	return len(k.channels)
}
