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
	Code      string `json:"code"`
	URL       string `json:"url"`
	Provider  string `json:"provider"`
	ZeroWidth bool   `json:"zero_width,omitempty"`
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
	// Synthetic messages (overlay-editor mocks and the public test stream)
	// enrich through the same cache path but must not record cache-operation or
	// provider-lookup metrics: a short editor test burst is enough to push the
	// AllChatEmoteCacheEfficiencyDeclining and AllChatMessageRateAnomaly alerts
	// over their thresholds, because the alert windows are small and baseline
	// traffic is low. Synthetic volume stays observable through api-gateway
	// request logs and testgen counters instead.
	synthetic := isSynthetic(msg.Metadata)

	// A per-user cache entry is only worth a key when the chatter can actually
	// have personal emotes to put in it. Drop the user id otherwise so the
	// lookup shares the channel-level entry outright.
	userID := msg.User.ID
	if synthetic || !hasPersonalEmotes(msg.Platform) {
		userID = ""
	}

	// Fetch emotes for the channel (with user context if available)
	thirdPartyEmotes, err := e.fetchEmotes(ctx, channelIdentifier, msg.Platform, userID, twitchChannel, seventvSetID, synthetic)
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
					ZeroWidth: emote.ZeroWidth,
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

func (e *Enricher) fetchEmotes(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string, synthetic bool) ([]cache.CachedEmote, error) {
	// When a per-overlay 7TV override is set, the cache key would need to include
	// it to stay correct. Overrides are rare per request volume — bypass the cache
	// entirely for these calls and skip cache writes too. The emote-service still
	// caches at its own layer.
	useCache := seventvSetID == "" && e.cache != nil

	if useCache {
		var entry cache.Entry
		var err error
		// Tracked apart from entry.Stale because the two halves need different
		// refreshes: only a refresh with no user id rewrites the channel key.
		var channelHalfStale bool
		if userID != "" {
			entry, channelHalfStale, err = e.personalPlusChannel(ctx, channel, userID)
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
			if e.processorMetrics != nil && !synthetic {
				e.processorMetrics.RecordEmoteCacheOperation("message-processor", "hit", "all")
			}
			if entry.Stale {
				e.refreshAsync(channel, platform, userID, twitchChannel)
			}
			// The refresh above carries the user id, so it rewrites only the
			// per-user half (fetchFromService seeds the channel key on a miss, not
			// on a stale hit). Refresh the channel key on its own key as well, or
			// it stays stale until it hard-expires and every message from every
			// chatter triggers another per-user refresh for the whole 12h grace.
			// refreshAsync keys on channel+userID, so this one is deduplicated
			// independently and stays at one in-flight fetch per channel.
			if channelHalfStale {
				e.refreshAsync(channel, platform, "", twitchChannel)
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
	if e.processorMetrics != nil && !synthetic {
		e.processorMetrics.RecordEmoteCacheOperation("message-processor", "miss", "all")
	}

	return e.fetchFromService(ctx, channel, platform, userID, twitchChannel, seventvSetID, useCache, synthetic)
}

// personalPlusChannel reassembles a chatter's full emote set from the two keys
// it is split across: the channel entry everyone shares, and the per-user entry
// holding only that chatter's own emotes (see splitPersonal).
//
// Both halves must be present to answer. A hit on one alone is reported as
// ErrCacheMiss so the caller re-fetches and rewrites the pair: the halves carry
// independent TTLs and either can be evicted first, and serving the channel
// half alone would silently drop the chatter's own emotes while serving the
// personal half alone would drop the channel's.
//
// The entry is stale if either half is, and the channel half's staleness is
// returned separately: a refresh carrying a user id rewrites only the personal
// key, so the caller needs to know when to refresh the channel key too.
func (e *Enricher) personalPlusChannel(ctx context.Context, channel, userID string) (cache.Entry, bool, error) {
	personal, err := e.cache.GetEntryWithUser(ctx, channel, userID)
	if err != nil {
		return cache.Entry{}, false, err
	}
	channelEntry, err := e.cache.GetEntry(ctx, channel)
	if err != nil {
		return cache.Entry{}, false, err
	}

	merged := make([]cache.CachedEmote, 0, len(channelEntry.Emotes)+len(personal.Emotes))
	merged = append(merged, channelEntry.Emotes...)
	merged = append(merged, personal.Emotes...)

	return cache.Entry{Emotes: merged, Stale: personal.Stale || channelEntry.Stale}, channelEntry.Stale, nil
}

// splitPersonal returns the emotes in merged that the channel entry does not
// already carry, i.e. the ones that are genuinely this chatter's.
//
// Only this remainder belongs under the per-user key. Storing the merged set
// there instead put a near-complete copy of the channel's emotes (384-1486 of
// them, ~100-230 KB) behind every chatter id for 13h, which is what filled
// Redis and started the eviction of untyped state (AllChatRedisEvictingKeys,
// 2026-11-18).
//
// Codes are the identity: the emote map Enrich builds is keyed by code, so two
// entries sharing a code are indistinguishable downstream and the channel one
// wins either way.
//
// merged can be SHORTER than channelEmotes: emote-service drops a provider's
// emotes when that provider times out, which its logs show happening routinely.
// The result is then empty, which is correct — a degraded response adds nothing
// the channel entry lacks.
func splitPersonal(merged, channelEmotes []cache.CachedEmote) []cache.CachedEmote {
	inChannel := make(map[string]struct{}, len(channelEmotes))
	for _, emote := range channelEmotes {
		inChannel[emote.Code] = struct{}{}
	}

	var personal []cache.CachedEmote
	for _, emote := range merged {
		if _, shared := inChannel[emote.Code]; !shared {
			personal = append(personal, emote)
		}
	}
	return personal
}

// seedChannelEntry fetches the channel's shared emote set — no user id, so no
// chatter's personal emotes in it — and writes it to the channel cache key.
//
// It exists because the per-user write needs a channel entry to subtract from
// (see splitPersonal) and a channel whose chatters all carry a user id never
// reaches the userID == "" branch that would otherwise create one.
//
// The route selection mirrors fetchFromService: GetEmotesForChannel takes no
// platform argument, so a non-Twitch platform has to go down the user-aware
// call with an empty user id to keep its platform (see the reasoning there).
func (e *Enricher) seedChannelEntry(ctx context.Context, channel, platform, twitchChannel string) ([]cache.CachedEmote, error) {
	var emotes []EmoteServiceEmote
	var err error
	if twitchChannel != "" || isNonTwitchPlatform(platform) {
		emotes, err = e.client.GetEmotesForChannelWithUser(ctx, channel, platform, "", twitchChannel, "")
	} else {
		emotes, err = e.client.GetEmotesForChannel(ctx, channel)
	}
	if err != nil {
		return nil, err
	}

	converted := convertToCached(emotes)
	// Never cache an empty result, for the reason given at the call site in
	// fetchFromService: a transient upstream failure would otherwise suppress
	// this channel's emotes for the entry's whole lifetime.
	if len(converted) == 0 {
		return nil, fmt.Errorf("emote service returned no channel emotes for %q", channel)
	}

	if err := e.cache.Set(ctx, channel, converted); err != nil {
		return nil, err
	}
	return converted, nil
}

// fetchFromService fetches emotes from the emote service, records lookup metrics,
// and (when writeCache is set) populates the cache. It is shared by the blocking
// cache-miss path and the background stale-refresh path.
func (e *Enricher) fetchFromService(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string, writeCache, synthetic bool) ([]cache.CachedEmote, error) {
	var thirdPartyEmotes []EmoteServiceEmote
	var err error

	// GetEmotesForChannel takes no platform argument, so it may only serve a
	// request with nothing for emote-service to branch on. A non-Twitch platform
	// alone is enough: emote-service uses it to add Twitch globals, to fall back
	// to the "global" BTTV/FFZ set instead of looking those providers up by a
	// platform id, and to pick the 7TV channel-connection path. Routing such a
	// request down the plain call drops the platform and degrades all three
	// silently.
	if userID != "" || twitchChannel != "" || seventvSetID != "" || isNonTwitchPlatform(platform) {
		thirdPartyEmotes, err = e.client.GetEmotesForChannelWithUser(ctx, channel, platform, userID, twitchChannel, seventvSetID)
	} else {
		thirdPartyEmotes, err = e.client.GetEmotesForChannel(ctx, channel)
	}

	if err != nil {
		return nil, err
	}

	// Record per-provider lookup results
	if e.processorMetrics != nil && !synthetic {
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
			// An empty split is still written. The entry's presence is what records
			// that this chatter has been looked up; without it "has no personal
			// emotes" is indistinguishable from "never asked", and the read path
			// would re-fetch on every message from the many chatters who have none.
			channelEntry, chErr := e.cache.GetEntry(ctx, channel)
			switch {
			case errors.Is(chErr, cache.ErrCacheMiss):
				// Seed it (see seedChannelEntry). Not from converted: that answers a
				// user-aware request, into which emote-service merges this chatter's
				// personal 7TV set (FetchCombinedEmotes), and storing it under the key
				// everyone reads would serve one chatter's emotes to the whole channel
				// for up to 18h. The extra fetch is paid once per cold channel,
				// against an emote service that has just cached this channel's
				// providers. Skipping the seed instead left such a channel caching
				// nothing at all — every message missing both keys and paying a 3s
				// fetch, which is worse than the memory it saved.
				channelEmotes, seedErr := e.seedChannelEntry(ctx, channel, platform, twitchChannel)
				if seedErr != nil {
					// Without a channel entry to subtract, the split cannot tell this
					// chatter's emotes from the channel's. Skip the per-user write and
					// leave the pair unwritten rather than store a full copy under it,
					// which is what filled Redis. The next message retries.
					e.logger.Warn("Failed to seed channel emote cache entry, skipping per-user write",
						zap.String("channel", channel),
						zap.String("user_id", userID),
						zap.Error(seedErr),
					)
					return converted, nil
				}
				channelEntry = cache.Entry{Emotes: channelEmotes}
			case chErr != nil:
				// A transport or decode error says nothing about whether the entry
				// exists, so neither seeding nor splitting against an empty set is
				// safe. Serve this message and leave the cache alone.
				e.logger.Warn("Failed to read channel emote entry, skipping per-user write",
					zap.String("channel", channel),
					zap.String("user_id", userID),
					zap.Error(chErr),
				)
				return converted, nil
			}

			personal := splitPersonal(converted, channelEntry.Emotes)
			if err := e.cache.SetWithUser(ctx, channel, userID, personal); err != nil {
				e.logger.Warn("Failed to populate emote cache",
					zap.String("channel", channel),
					zap.String("user_id", userID),
					zap.Error(err),
				)
			} else {
				e.logger.Debug("Emote cache populated (with user)",
					zap.String("channel", channel),
					zap.String("user_id", userID),
					zap.Int("count", len(personal)),
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
		// Refreshes keep counting as real cache traffic: they run against the same
		// key a real message would have served, so provider lookups here reflect
		// genuine cache health.
		if _, err := e.fetchFromService(ctx, channel, platform, userID, twitchChannel, "", true, false); err != nil {
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

// hasPersonalEmotes reports whether a chatter on this platform can have a
// personal emote set we are able to resolve. 7TV is the only provider of
// per-user emotes, via /v3/users/{platform}/{id} (FetchUserEmotes in
// emote-service), and that lookup accepts only Twitch and Kick chatter ids
// (an unlinked Kick id is a 404, a TikTok username or a YouTube chatter id is
// always a 400). For every other platform the merged response is
// byte-identical to the channel-level one, so a per-user cache key stored a
// redundant full-size copy of the channel emote set (~100-230 KB) per chatter
// per channel for 13h, which filled Redis (AllChatRedisMemoryNearLimit,
// 2026-11-18).
//
// This is about CHATTER ids only. The same 7TV endpoint does serve YouTube and
// Kick CHANNEL connections — see fetchPlatformConnectionEmotes in
// emote-service — so the channel lookup is unaffected by this predicate.
func hasPersonalEmotes(platform string) bool {
	return platform == "twitch" || platform == "kick"
}

// isNonTwitchPlatform reports whether emote-service has platform-specific
// handling to do for this request. It mirrors the identically named condition
// in emote-service's own handler (handlers/emote.go), including treating an
// unknown platform as nothing to branch on.
func isNonTwitchPlatform(platform string) bool {
	return platform != "" && platform != "twitch"
}

// isSynthetic reports whether a message originates from a synthetic source —
// the overlay-editor mock messages or the public test-stream generator. Both
// set a metadata flag at build time (buildMockMessage in cmd/main.go,
// newMessage in testgen/content.go).
func isSynthetic(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	if v, ok := metadata["mock"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	if v, ok := metadata["test_stream"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	return false
}

func convertToCached(emotes []EmoteServiceEmote) []cache.CachedEmote {
	converted := make([]cache.CachedEmote, 0, len(emotes))
	for _, emote := range emotes {
		converted = append(converted, cache.CachedEmote{
			Code:      emote.Code,
			Provider:  emote.Provider,
			URL:       emote.URL,
			ZeroWidth: emote.ZeroWidth,
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
