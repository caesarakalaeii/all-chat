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

package altstream

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/altstream-listener/metrics"
	"go.uber.org/zap"
)

// The public developer API (altstream.online/documentation, External
// Developer API tab) allows 60 REST requests per minute per IP. The listener
// spends half of that, shared by the lastId bootstrap and the emote catalogs
// of every channel; SSE connects do not count against it.
const (
	restInterval   = 2 * time.Second // 30 requests per minute
	restBurst      = 10              // lets a pod starting with a few channels bootstrap at once
	restTimeout    = 15 * time.Second
	emoteRefresh   = 10 * time.Minute
	sentinelLastID = "none" // any lastId that matches no message; see latestID
)

// getJSON fetches one REST resource through the shared rate limiter.
func (c *Client) getJSON(ctx context.Context, endpoint, path string, query url.Values, out any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	u := c.baseURL + path
	if query != nil {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.rest.Do(req)
	if err != nil {
		metrics.IncRESTRequest(endpoint, "error")
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.IncRESTRequest(endpoint, fmt.Sprintf("http_%d", resp.StatusCode))
		return fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}
	metrics.IncRESTRequest(endpoint, "ok")
	return json.NewDecoder(resp.Body).Decode(out)
}

// latestID returns the id to send as lastId on the first connect. The stream
// requires a non-empty lastId: without one it sends only pings, forever. The
// newest message id makes it send only new messages; when the channel has no
// chat yet (or the lookup fails) a sentinel that matches nothing makes it
// replay the newest 1 to 3 messages, which the catch-up filter drops.
func (c *Client) latestID(ctx context.Context, channel string) string {
	var history []Event
	err := c.getJSON(ctx, "chat", "/api/chat", url.Values{"channel": {channel}, "limit": {"1"}}, &history)
	if err != nil {
		c.log.Warn("lastId bootstrap failed, using sentinel", zap.String("channel", channel), zap.Error(err))
		return sentinelLastID
	}
	if len(history) == 0 || history[len(history)-1].ID == "" {
		return sentinelLastID
	}
	return history[len(history)-1].ID
}

// fetchEmotes returns a channel's emote catalog as name to absolute URL.
func (c *Client) fetchEmotes(ctx context.Context, channel string) (map[string]string, error) {
	var body struct {
		Data struct {
			Emotes []Emote `json:"emotes"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, "emotes", "/api/v1/channels/"+url.PathEscape(channel)+"/emotes", nil, &body); err != nil {
		return nil, err
	}
	cat := make(map[string]string, len(body.Data.Emotes))
	for _, e := range body.Data.Emotes {
		if e.Name == "" || e.URL == "" {
			continue
		}
		if strings.HasPrefix(e.URL, "/") { // catalog URLs are site-relative
			e.URL = c.baseURL + e.URL
		}
		cat[e.Name] = e.URL
	}
	return cat, nil
}

// updateEmotes stores a fresh catalog for a channel. A failed fetch keeps
// the previous one.
func (c *Client) updateEmotes(ctx context.Context, channel string) {
	cat, err := c.fetchEmotes(ctx, channel)
	if err != nil {
		if ctx.Err() == nil {
			c.log.Warn("emote catalog fetch failed", zap.String("channel", channel), zap.Error(err))
		}
		return
	}
	c.emotes.set(channel, cat)
}

// refreshEmotes refreshes a channel's catalog while its stream runs and
// removes it from the shared store when the stream stops.
func (c *Client) refreshEmotes(ctx context.Context, channel string) {
	defer c.emotes.set(channel, nil)
	ticker := time.NewTicker(emoteRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.updateEmotes(ctx, channel)
		}
	}
}

// emoteToken matches a :name: token in chat text.
var emoteToken = regexp.MustCompile(`:([^\s:]+):`)

// emoteStore is the one name to URL map shared by every watched channel: a
// viewer may use any emote of any channel they follow, so a token in one
// channel's chat can belong to another channel. There is no anonymous global
// lookup, so only the catalogs of channels this pod watches can resolve.
type emoteStore struct {
	mu        sync.RWMutex
	byChannel map[string]map[string]string
	merged    map[string]string
}

func newEmoteStore() *emoteStore {
	return &emoteStore{byChannel: map[string]map[string]string{}, merged: map[string]string{}}
}

// set replaces a channel's catalog; nil removes it.
// ponytail: on a name clash between channels an arbitrary one wins; prefer the
// message's own channel if clashes ever show up.
func (s *emoteStore) set(channel string, cat map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cat == nil {
		delete(s.byChannel, channel)
	} else {
		s.byChannel[channel] = cat
	}
	merged := make(map[string]string)
	for _, c := range s.byChannel {
		maps.Copy(merged, c)
	}
	s.merged = merged
}

// resolve lists the known :name: tokens of text once each, in order of first
// appearance. Unknown tokens stay plain text.
func (s *emoteStore) resolve(text string) []Emote {
	out := []Emote{}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range emoteToken.FindAllStringSubmatch(text, -1) {
		u, ok := s.merged[m[1]]
		if !ok {
			continue
		}
		dup := false
		for _, e := range out {
			dup = dup || e.Name == m[1]
		}
		if !dup {
			out = append(out, Emote{Name: m[1], URL: u})
		}
	}
	return out
}
