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

// Package altstream reads Altstream (altstream.online) live chat: one SSE
// stream per watched channel, plus the public REST API for the lastId
// bootstrap and the channel emote catalogs. No authentication is needed.
// See docs/adr/0082-altstream-public-sse-chat.md.
package altstream

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caesar/all-chat/services/altstream-listener/metrics"
	"github.com/caesar/all-chat/shared/listener"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	// DefaultBaseURL is the production site; ALTSTREAM_BASE_URL overrides it.
	DefaultBaseURL = "https://altstream.online"

	// staleAfter: the server sends `: ping` about every 25s, so a stream
	// that delivers nothing at all for three ping periods is dead.
	staleAfter = 75 * time.Second

	// catchUpGrace: on connect the stream replays the newest 1 to 3 existing
	// messages first. Events sent before (connect time - grace) are such
	// replays and must not reach overlays again. The grace absorbs
	// Altstream's whole-second sentAt and some clock skew.
	catchUpGrace = 10 * time.Second

	// seenCapacity bounds the per-channel dedupe set. Replays are at most a
	// few messages, so the last few hundred ids are plenty.
	seenCapacity = 256

	// healthyConnection: a stream that stayed up this long resets backoff.
	healthyConnection = time.Minute
)

// validChannel is the accepted Altstream username shape (observed charset is
// [a-z0-9_]); anything else never reaches a URL.
var validChannel = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// Handler receives every new event of a watched channel.
type Handler func(channel string, ev *Event)

// Client runs one SSE stream per watched channel. Each stream reconnects on
// its own with backoff, resuming from the last event id it saw.
type Client struct {
	baseURL string
	handler Handler
	log     *zap.Logger
	stream  *http.Client // no timeout: SSE responses never end
	rest    *http.Client
	limiter *rate.Limiter // shared by every REST call of every channel
	emotes  *emoteStore

	// Overridable by tests.
	staleAfter   time.Duration
	catchUpGrace time.Duration
	backoff      func(attempt int) time.Duration

	mu      sync.Mutex
	streams map[string]*stream
}

type stream struct {
	cancel    context.CancelFunc
	connected atomic.Bool
}

// NewClient creates a client against baseURL (DefaultBaseURL in production).
func NewClient(baseURL string, handler Handler, log *zap.Logger) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		handler:      handler,
		log:          log,
		stream:       &http.Client{},
		rest:         &http.Client{Timeout: restTimeout},
		limiter:      rate.NewLimiter(rate.Every(restInterval), restBurst),
		emotes:       newEmoteStore(),
		staleAfter:   staleAfter,
		catchUpGrace: catchUpGrace,
		backoff:      listener.JitteredBackoff,
		streams:      make(map[string]*stream),
	}
}

// Connect starts the stream of a channel. It is a no-op while the stream
// runs; connection failures are retried in the background.
func (c *Client) Connect(channel string) error {
	channel = strings.ToLower(channel)
	if !validChannel.MatchString(channel) {
		return fmt.Errorf("invalid altstream channel %q", channel)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.streams[channel]; ok {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	st := &stream{cancel: cancel}
	c.streams[channel] = st
	go c.run(ctx, channel, st)
	return nil
}

// Disconnect stops a channel's stream.
func (c *Client) Disconnect(channel string) {
	channel = strings.ToLower(channel)
	c.mu.Lock()
	st, ok := c.streams[channel]
	delete(c.streams, channel)
	c.mu.Unlock()
	if ok {
		st.cancel()
	}
}

// DisconnectAll stops every stream (shutdown path).
func (c *Client) DisconnectAll() {
	c.mu.Lock()
	streams := c.streams
	c.streams = make(map[string]*stream)
	c.mu.Unlock()
	for _, st := range streams {
		st.cancel()
	}
}

// IsConnected reports whether the channel's SSE response is currently open.
func (c *Client) IsConnected(channel string) bool {
	c.mu.Lock()
	st, ok := c.streams[strings.ToLower(channel)]
	c.mu.Unlock()
	return ok && st.connected.Load()
}

// cursor is the per-channel resume state, owned by the stream goroutine.
type cursor struct {
	lastID string
	since  time.Time // events sent before this are catch-up replays
	seen   *idSet
}

// run bootstraps lastId, keeps the emote catalog fresh and holds the stream
// open, reconnecting with backoff until ctx ends.
func (c *Client) run(ctx context.Context, channel string, st *stream) {
	cur := &cursor{since: time.Now().Add(-c.catchUpGrace), seen: newIDSet(seenCapacity)}
	cur.lastID = c.latestID(ctx, channel)
	c.updateEmotes(ctx, channel) // before the first connect, so the first messages resolve
	go c.refreshEmotes(ctx, channel)

	for attempt := 0; ctx.Err() == nil; attempt++ {
		started := time.Now()
		reason, lastActivity := c.readStream(ctx, channel, st, cur)
		if ctx.Err() != nil {
			return
		}
		metrics.IncReconnect(reason)
		if !lastActivity.IsZero() {
			// Whatever the reconnect replays that was sent after we last
			// heard from the server was missed in the gap: let it through.
			cur.since = lastActivity.Add(-c.catchUpGrace)
		}
		if time.Since(started) > healthyConnection {
			attempt = 0
		}
		wait := c.backoff(attempt)
		c.log.Info("altstream stream dropped, reconnecting",
			zap.String("channel", channel), zap.String("reason", reason), zap.Duration("backoff", wait))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// readStream holds one SSE connection open and returns why it ended plus the
// time anything (data or ping) last arrived.
func (c *Client) readStream(ctx context.Context, channel string, st *stream, cur *cursor) (reason string, lastActivity time.Time) {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stale atomic.Bool
	watchdog := time.AfterFunc(c.staleAfter, func() { stale.Store(true); cancel() })
	defer watchdog.Stop()

	q := url.Values{"channel": {channel}, "lastId": {cur.lastID}}
	req, err := http.NewRequestWithContext(connCtx, http.MethodGet, c.baseURL+"/api/chat/stream?"+q.Encode(), nil)
	if err != nil {
		return "request_error", lastActivity
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.stream.Do(req)
	if err != nil {
		if stale.Load() {
			return "stale", lastActivity
		}
		c.log.Warn("altstream stream connect failed", zap.String("channel", channel), zap.Error(err))
		return "connect_error", lastActivity
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Transient upstream errors (a 502 during an Altstream deploy): back
		// off and retry like any other drop.
		c.log.Warn("altstream stream refused", zap.String("channel", channel), zap.Int("status", resp.StatusCode))
		return fmt.Sprintf("http_%d", resp.StatusCode), lastActivity
	}

	st.connected.Store(true)
	metrics.AddConnectedStreams(1)
	defer func() {
		st.connected.Store(false)
		metrics.AddConnectedStreams(-1)
	}()

	// Every frame observed so far is one `data: [...]` line, so each data
	// line is parsed on its own; `: ping` comments only feed the watchdog.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		watchdog.Reset(c.staleAfter)
		lastActivity = time.Now()
		payload, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		events, err := parseFrame(payload)
		if err != nil {
			c.log.Debug("dropping malformed altstream frame", zap.String("channel", channel), zap.Error(err))
			metrics.IncDropped("malformed_frame")
			continue
		}
		for _, ev := range events {
			c.deliver(channel, ev, cur)
		}
	}
	switch {
	case stale.Load():
		return "stale", lastActivity
	case scanner.Err() != nil:
		return "read_error", lastActivity
	default:
		return "eof", lastActivity
	}
}

// deliver applies the resume, dedupe and catch-up rules to one event and
// hands survivors to the handler.
func (c *Client) deliver(channel string, ev *Event, cur *cursor) {
	if ev.ID != "" {
		// Track every id, replays and dropped types included: the server
		// compares lastId against its newest event of any type.
		cur.lastID = ev.ID
		if !cur.seen.add(ev.ID) {
			metrics.IncDropped("duplicate")
			return
		}
	}
	if ev.SentAt > 0 && time.UnixMilli(ev.SentAt).Before(cur.since) {
		metrics.IncDropped("catch_up")
		return
	}
	if ev.Type == TypeMessage || ev.Type == TypeHighlight {
		ev.Emotes = c.emotes.resolve(ev.ChatText())
	}
	if c.handler != nil {
		c.handler(channel, ev)
	}
}

// idSet remembers the last n ids in a ring.
type idSet struct {
	ids  map[string]struct{}
	ring []string
	next int
}

func newIDSet(n int) *idSet {
	return &idSet{ids: make(map[string]struct{}, n), ring: make([]string, n)}
}

// add records id and reports whether it was new.
func (s *idSet) add(id string) bool {
	if _, ok := s.ids[id]; ok {
		return false
	}
	delete(s.ids, s.ring[s.next])
	s.ring[s.next] = id
	s.ids[id] = struct{}{}
	s.next = (s.next + 1) % len(s.ring)
	return true
}
