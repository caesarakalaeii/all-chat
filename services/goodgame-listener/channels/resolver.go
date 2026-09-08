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

package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StreamBaseURL is the GoodGame API v4 stream endpoint used to resolve a
// channel slug ("Miker") to the numeric chat channel id ("5").
// Source: GoodGame/API Streams/stream_api.md and goodgame.ru/api/4/stream/<key>
// (verified live 2026-09; the payload field is `id`, the request key echoes
// back as `channelkey`).
const StreamBaseURL = "https://goodgame.ru/api/4/stream/"

// resolveCache TTL. Numeric channel ids are effectively permanent; the TTL
// only bounds how long a stale entry survives a channel rename. On a resolve
// miss the cached entry is dropped and re-fetched immediately.
const resolveCacheTTL = 24 * time.Hour

// StreamResponse is the subset of GET /api/4/stream/<key> the resolver reads.
type StreamResponse struct {
	// ID is the numeric channel/stream id — the id the chat WebSocket's
	// join/frame payloads use as channel_id. Verified live: /api/4/stream/miker
	// returns {"id":5,...} and chat frames for that channel agree.
	ID int64 `json:"id"`
	// ChannelKey echoes the symbolic key from the request.
	ChannelKey string `json:"channelkey"`
	// Online / Status are informational; a channel can be joined offline.
	Online bool `json:"online"`
}

// NotFoundError marks a stream key that does not exist on GoodGame.
type NotFoundError struct{ Key string }

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("goodgame stream key %q not found", e.Key)
}

// Resolver maps a GoodGame channel slug (as entered by the streamer into
// overlay_chat_sources.channel_identifier) to the numeric chat channel id
// the WebSocket protocol requires. Results are cached in-process with a TTL;
// nothing is persisted (the kick-listener stores chatroom_id in the DB config
// JSONB, but the numeric id resolution here is cheap and self-healing, so the
// listener keeps the DB column slug-only).
type Resolver struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger

	mu      sync.Mutex
	entries map[string]resolveEntry
}

type resolveEntry struct {
	id        int64
	expiresAt time.Time
}

// NewResolver builds a resolver against the production GoodGame API.
func NewResolver(logger *zap.Logger) *Resolver {
	return NewResolverWithBase(StreamBaseURL, &http.Client{Timeout: 10 * time.Second}, logger)
}

// NewResolverWithBase builds a resolver against an explicit API base URL
// (tests point this at a fake server).
func NewResolverWithBase(baseURL string, client *http.Client, logger *zap.Logger) *Resolver {
	return &Resolver{
		baseURL: baseURL,
		client:  client,
		logger:  logger,
		entries: make(map[string]resolveEntry),
	}
}

// Resolve returns the numeric chat channel id for a channel slug.
// Slugs are normalised case-insensitively (GoodGame treats keys
// case-insensitively across its own APIs).
func (r *Resolver) Resolve(ctx context.Context, slug string) (int64, error) {
	r.mu.Lock()
	entry, ok := r.entries[slug]
	if ok && time.Now().Before(entry.expiresAt) {
		r.mu.Unlock()
		return entry.id, nil
	}
	if ok {
		// Expired — drop the stale entry so a permanent rename is not shadowed
		// by a cached id.
		delete(r.entries, slug)
	}
	r.mu.Unlock()

	id, err := r.fetch(ctx, slug)
	if err != nil {
		return 0, err
	}

	r.mu.Lock()
	r.entries[slug] = resolveEntry{id: id, expiresAt: time.Now().Add(resolveCacheTTL)}
	r.mu.Unlock()

	return id, nil
}

// ResolveFresh forces a network resolve, refreshing (or establishing) the
// cache entry. Called when a live join fails with an unknown-channel error —
// the cached id is then stale (rename/removal) and must not be retried.
func (r *Resolver) ResolveFresh(ctx context.Context, slug string) (int64, error) {
	r.mu.Lock()
	delete(r.entries, slug)
	r.mu.Unlock()

	return r.Resolve(ctx, slug)
}

func (r *Resolver) fetch(ctx context.Context, slug string) (int64, error) {
	url := r.baseURL + slug

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create resolve request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AllChat/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve stream key %s: %w", slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, &NotFoundError{Key: slug}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 0, fmt.Errorf("resolve API returned %d: %s", resp.StatusCode, string(body))
	}

	var stream StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&stream); err != nil {
		return 0, fmt.Errorf("failed to decode resolve response for %s: %w", slug, err)
	}
	if stream.ID == 0 {
		return 0, fmt.Errorf("resolve response for %s carries no channel id", slug)
	}

	r.logger.Info("Resolved GoodGame channel id",
		zap.String("slug", slug),
		zap.Int64("channel_id", stream.ID),
	)
	return stream.ID, nil
}

// StringID renders a numeric id in the wire format (decimal string).
func StringID(id int64) string {
	return strconv.FormatInt(id, 10)
}
