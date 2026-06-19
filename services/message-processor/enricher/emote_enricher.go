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

package enricher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/shared/metrics"
	"go.uber.org/zap"
)

// EmoteServiceClient is an interface for calling the Emote Service
type EmoteServiceClient interface {
	GetEmotesForChannel(ctx context.Context, channel string) ([]EmoteServiceEmote, error)
	GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]EmoteServiceEmote, error)
}

// EmoteServiceEmote represents an emote from the Emote Service
type EmoteServiceEmote struct {
	Code     string `json:"code"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
}

// EmoteServiceResponse is the response from Emote Service
type EmoteServiceResponse struct {
	Channel string              `json:"channel"`
	Emotes  []EmoteServiceEmote `json:"emotes"`
}

// HTTPEmoteClient implements EmoteServiceClient using HTTP
type HTTPEmoteClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// NewHTTPEmoteClient creates a new HTTP client for Emote Service
func NewHTTPEmoteClient(baseURL string, logger *zap.Logger) *HTTPEmoteClient {
	return &HTTPEmoteClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// GetEmotesForChannel fetches all emotes for a channel from the Emote Service
func (c *HTTPEmoteClient) GetEmotesForChannel(ctx context.Context, channel string) ([]EmoteServiceEmote, error) {
	escapedChannel := url.PathEscape(channel)
	endpoint, err := url.JoinPath(c.baseURL, "emotes", "channel", escapedChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to build emote service url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call emote service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("emote service returned status %d", resp.StatusCode)
	}

	var emoteResp EmoteServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&emoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return emoteResp.Emotes, nil
}

// GetEmotesForChannelWithUser fetches emotes for a channel including user-specific emotes.
// twitchChannel is an optional hint from a sibling Twitch source on the same overlay,
// enabling 7TV channel emote lookup for non-Twitch platforms. seventvSetID is an
// optional per-overlay 7TV emote-set override that's merged into the result.
func (c *HTTPEmoteClient) GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]EmoteServiceEmote, error) {
	escapedChannel := url.PathEscape(channel)
	endpoint, err := url.JoinPath(c.baseURL, "emotes", "channel", escapedChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to build emote service url: %w", err)
	}

	// Parse URL and add query parameters
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	query := parsedURL.Query()
	if userID != "" {
		query.Set("user_id", userID)
	}
	if platform != "" {
		query.Set("platform", platform)
	}
	if twitchChannel != "" {
		query.Set("twitch_channel", twitchChannel)
	}
	if seventvSetID != "" {
		query.Set("seventv_set_id", seventvSetID)
	}
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call emote service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("emote service returned status %d", resp.StatusCode)
	}

	var emoteResp EmoteServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&emoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return emoteResp.Emotes, nil
}

// Enricher enriches messages with third-party emotes
type Enricher struct {
	client           EmoteServiceClient
	cache            cache.Store
	processorMetrics *metrics.ProcessorMetrics
	logger           *zap.Logger
	// refreshing tracks cache keys with an in-flight background refresh so a burst
	// of messages on a channel with a stale entry only triggers one re-fetch.
	refreshing sync.Map
}

// NewEnricher creates a new emote enricher
func NewEnricher(client EmoteServiceClient, cacheStore cache.Store, logger *zap.Logger) *Enricher {
	return &Enricher{
		client: client,
		cache:  cacheStore,
		logger: logger,
	}
}

// SetMetrics sets the processor metrics instance for recording emote enrichment telemetry.
func (e *Enricher) SetMetrics(m *metrics.ProcessorMetrics) {
	e.processorMetrics = m
}

// Enrich adds third-party emotes (7TV, BTTV, FFZ) to the message
func (e *Enricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	channelIdentifier := msg.ChannelID
	if msg.Platform == "twitch" {
		if roomID, ok := msg.Metadata["twitch_room_id"]; ok {
			switch v := roomID.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					channelIdentifier = v
				}
			case fmt.Stringer:
				if s := strings.TrimSpace(v.String()); s != "" {
					channelIdentifier = s
				}
			default:
				if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
					channelIdentifier = s
				}
			}
		}
		// Default to "global" for Twitch if no channel is specified
		// This ensures Twitch global emotes (like PogChamp) work in mock messages
		if strings.TrimSpace(channelIdentifier) == "" {
			channelIdentifier = "global"
		}
	}
	// Extract twitch channel hint from metadata (set by main.go for non-Twitch overlays
	// that have a sibling Twitch source)
	var twitchChannel string
	if hint, ok := msg.Metadata["twitch_channel_hint"]; ok {
		if s, ok := hint.(string); ok {
			twitchChannel = s
		}
	}
	// Per-overlay 7TV emote-set override (set by main.go from overlay_configs).
	var seventvSetID string
	if hint, ok := msg.Metadata["seventv_emote_set_id"]; ok {
		if s, ok := hint.(string); ok {
			seventvSetID = s
		}
	}

	// Fetch emotes for the channel (with user context if available)
	thirdPartyEmotes, err := e.fetchEmotes(ctx, channelIdentifier, msg.Platform, msg.User.ID, twitchChannel, seventvSetID)
	if err != nil {
		// Don't fail the message if emote enrichment fails
		e.logger.Warn("Failed to fetch emotes, skipping enrichment",
			zap.String("channel", msg.ChannelID),
			zap.String("user_id", msg.User.ID),
			zap.Error(err),
		)
		return nil
	}

	// Build a map of emote code -> emote for quick lookup
	emoteMap := make(map[string]cache.CachedEmote)
	for _, emote := range thirdPartyEmotes {
		emoteMap[emote.Code] = emote
	}

	// Tokenize message text and find matching emotes
	words := strings.Fields(msg.Message.Text)
	occurrences := make(map[string]int)
	for _, word := range words {
		occurrence := occurrences[word]
		occurrences[word]++

		if emote, found := emoteMap[word]; found {
			// Calculate positions in the original text
			position := e.findWordPosition(msg.Message.Text, word, occurrence)
			if position != nil {
				msg.Message.Emotes = append(msg.Message.Emotes, models.Emote{
					Code:      emote.Code,
					Provider:  emote.Provider,
					URL:       emote.URL,
					Positions: [][]int{position},
				})
			}
		}
	}

	e.logger.Debug("Enriched message with emotes",
		zap.String("channel", msg.ChannelID),
		zap.Int("third_party_emotes", len(thirdPartyEmotes)),
		zap.Int("total_emotes", len(msg.Message.Emotes)),
	)

	return nil
}

func (e *Enricher) fetchEmotes(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]cache.CachedEmote, error) {
	// When a per-overlay 7TV override is set, the cache key would need to include
	// it to stay correct. Overrides are rare per request volume — bypass the cache
	// entirely for these calls and skip cache writes too. The emote-service still
	// caches at its own layer.
	useCache := seventvSetID == "" && e.cache != nil

	if useCache {
		var entry cache.Entry
		var err error
		if userID != "" {
			entry, err = e.cache.GetEntryWithUser(ctx, channel, userID)
		} else {
			entry, err = e.cache.GetEntry(ctx, channel)
		}

		switch {
		case err == nil:
			// Cache hit. Serve the entry immediately; if it's past its freshness
			// window, refresh it in the background so the next message is fresh
			// without ever blocking this one on the emote service.
			e.logger.Debug("Emote cache hit",
				zap.String("channel", channel),
				zap.String("user_id", userID),
				zap.Int("count", len(entry.Emotes)),
				zap.Bool("stale", entry.Stale),
			)
			if e.processorMetrics != nil {
				e.processorMetrics.RecordEmoteCacheOperation("message-processor", "hit", "all")
			}
			if entry.Stale {
				e.refreshAsync(channel, platform, userID, twitchChannel)
			}
			return entry.Emotes, nil
		case !errors.Is(err, cache.ErrCacheMiss):
			e.logger.Warn("Emote cache error",
				zap.String("channel", channel),
				zap.String("user_id", userID),
				zap.Error(err),
			)
		}
	}

	// True cache miss (no entry at all) — record and fetch from the emote service
	// synchronously, since we have nothing to serve yet.
	if e.processorMetrics != nil {
		e.processorMetrics.RecordEmoteCacheOperation("message-processor", "miss", "all")
	}

	return e.fetchFromService(ctx, channel, platform, userID, twitchChannel, seventvSetID, useCache)
}

// fetchFromService fetches emotes from the emote service, records lookup metrics,
// and (when writeCache is set) populates the cache. It is shared by the blocking
// cache-miss path and the background stale-refresh path.
func (e *Enricher) fetchFromService(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string, writeCache bool) ([]cache.CachedEmote, error) {
	var thirdPartyEmotes []EmoteServiceEmote
	var err error

	if userID != "" || twitchChannel != "" || seventvSetID != "" {
		thirdPartyEmotes, err = e.client.GetEmotesForChannelWithUser(ctx, channel, platform, userID, twitchChannel, seventvSetID)
	} else {
		thirdPartyEmotes, err = e.client.GetEmotesForChannel(ctx, channel)
	}

	if err != nil {
		return nil, err
	}

	// Record per-provider lookup results
	if e.processorMetrics != nil {
		providersSeen := make(map[string]bool)
		for _, emote := range thirdPartyEmotes {
			provider := strings.ToLower(emote.Provider)
			if provider == "" {
				provider = "unknown"
			}
			if !providersSeen[provider] {
				providersSeen[provider] = true
				e.processorMetrics.RecordEmoteLookup("message-processor", provider, "hit")
			}
		}
		// If no emotes returned from API, record a generic miss
		if len(thirdPartyEmotes) == 0 {
			e.processorMetrics.RecordEmoteLookup("message-processor", "all", "miss")
		}
	}

	converted := convertToCached(thirdPartyEmotes)

	// Never cache an empty result. A cold or transient upstream failure (e.g. an
	// expired Twitch app token) can make the emote service briefly return no
	// emotes; caching that would suppress emotes for this channel/user for the
	// full stale-while-revalidate lifetime (softTTL + stale grace) and serve it
	// without revalidating through the freshness window. Leaving it uncached lets
	// the next message re-fetch and pick up emotes as soon as the upstream
	// recovers. The emote service has its own cache, so the retry is cheap.
	if writeCache && len(converted) > 0 {
		if userID != "" {
			if err := e.cache.SetWithUser(ctx, channel, userID, converted); err != nil {
				e.logger.Warn("Failed to populate emote cache",
					zap.String("channel", channel),
					zap.String("user_id", userID),
					zap.Error(err),
				)
			} else {
				e.logger.Debug("Emote cache populated (with user)",
					zap.String("channel", channel),
					zap.String("user_id", userID),
					zap.Int("count", len(converted)),
				)
			}
		} else {
			if err := e.cache.Set(ctx, channel, converted); err != nil {
				e.logger.Warn("Failed to populate emote cache",
					zap.String("channel", channel),
					zap.Error(err),
				)
			} else {
				e.logger.Debug("Emote cache populated",
					zap.String("channel", channel),
					zap.Int("count", len(converted)),
				)
			}
		}
	}

	return converted, nil
}

// refreshAsync re-fetches emotes for a stale cache entry in the background and
// repopulates the cache. It is rate-limited to one in-flight refresh per cache
// key so a burst of messages doesn't stampede the emote service. The refresh
// runs on a fresh context independent of any message so it can't be cancelled
// by the message that triggered it returning.
func (e *Enricher) refreshAsync(channel, platform, userID, twitchChannel string) {
	key := channel + "\x00" + userID
	if _, inFlight := e.refreshing.LoadOrStore(key, struct{}{}); inFlight {
		return
	}

	go func() {
		defer e.refreshing.Delete(key)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// seventvSetID is always empty here: stale-while-revalidate only runs for
		// cacheable lookups, and per-overlay overrides bypass the cache entirely.
		if _, err := e.fetchFromService(ctx, channel, platform, userID, twitchChannel, "", true); err != nil {
			e.logger.Warn("Background emote cache refresh failed",
				zap.String("channel", channel),
				zap.String("user_id", userID),
				zap.Error(err),
			)
		} else {
			e.logger.Debug("Background emote cache refresh complete",
				zap.String("channel", channel),
				zap.String("user_id", userID),
			)
		}
	}()
}

func convertToCached(emotes []EmoteServiceEmote) []cache.CachedEmote {
	converted := make([]cache.CachedEmote, 0, len(emotes))
	for _, emote := range emotes {
		converted = append(converted, cache.CachedEmote{
			Code:     emote.Code,
			Provider: emote.Provider,
			URL:      emote.URL,
		})
	}
	return converted
}

// findWordPosition finds the position of a word in the text
// occurrence specifies which occurrence of the word to find (0-indexed)
func (e *Enricher) findWordPosition(text, word string, occurrence int) []int {
	currentOccurrence := 0
	pos := 0

	for {
		idx := strings.Index(text[pos:], word)
		if idx == -1 {
			return nil
		}

		// Check if this is a word boundary (not part of another word)
		actualPos := pos + idx
		if e.isWordBoundary(text, actualPos, len(word)) {
			if currentOccurrence == occurrence {
				return []int{actualPos, actualPos + len(word) - 1}
			}
			currentOccurrence++
		}

		pos = actualPos + 1
	}
}

// isWordBoundary checks if the substring at pos is a complete word
func (e *Enricher) isWordBoundary(text string, pos, length int) bool {
	// Check before
	if pos > 0 && !e.isBoundaryChar(text[pos-1]) {
		return false
	}

	// Check after
	endPos := pos + length
	if endPos < len(text) && !e.isBoundaryChar(text[endPos]) {
		return false
	}

	return true
}

// isBoundaryChar checks if a character is a word boundary
func (e *Enricher) isBoundaryChar(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == ',' || c == '.' || c == '!' || c == '?'
}
