package enricher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/shared/metrics"
	"go.uber.org/zap"
)

// EmoteServiceClient is an interface for calling the Emote Service
type EmoteServiceClient interface {
	GetEmotesForChannel(ctx context.Context, channel string) ([]EmoteServiceEmote, error)
	GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID string) ([]EmoteServiceEmote, error)
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

// GetEmotesForChannelWithUser fetches emotes for a channel including user-specific emotes
func (c *HTTPEmoteClient) GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID string) ([]EmoteServiceEmote, error) {
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
	client          EmoteServiceClient
	cache           cache.Store
	processorMetrics *metrics.ProcessorMetrics
	logger          *zap.Logger
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
	// Fetch emotes for the channel (with user context if available)
	thirdPartyEmotes, err := e.fetchEmotes(ctx, channelIdentifier, msg.Platform, msg.User.ID)
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

func (e *Enricher) fetchEmotes(ctx context.Context, channel, platform, userID string) ([]cache.CachedEmote, error) {
	// If user ID is provided, use user-specific cache
	if userID != "" && e.cache != nil {
		if cached, err := e.cache.GetWithUser(ctx, channel, userID); err == nil {
			e.logger.Debug("Emote cache hit (with user)",
				zap.String("channel", channel),
				zap.String("user_id", userID),
				zap.Int("count", len(cached)),
			)
			if e.processorMetrics != nil {
				e.processorMetrics.RecordEmoteCacheOperation("message-processor", "hit", "all")
			}
			return cached, nil
		} else if !errors.Is(err, cache.ErrCacheMiss) {
			e.logger.Warn("Emote cache error",
				zap.String("channel", channel),
				zap.String("user_id", userID),
				zap.Error(err),
			)
		}
	} else if e.cache != nil {
		// No user ID, use regular cache
		if cached, err := e.cache.Get(ctx, channel); err == nil {
			e.logger.Debug("Emote cache hit",
				zap.String("channel", channel),
				zap.Int("count", len(cached)),
			)
			if e.processorMetrics != nil {
				e.processorMetrics.RecordEmoteCacheOperation("message-processor", "hit", "all")
			}
			return cached, nil
		} else if !errors.Is(err, cache.ErrCacheMiss) {
			e.logger.Warn("Emote cache error",
				zap.String("channel", channel),
				zap.Error(err),
			)
		}
	}

	// Cache miss — record and fetch from emote service
	if e.processorMetrics != nil {
		e.processorMetrics.RecordEmoteCacheOperation("message-processor", "miss", "all")
	}

	// Fetch from emote service
	var thirdPartyEmotes []EmoteServiceEmote
	var err error

	if userID != "" {
		thirdPartyEmotes, err = e.client.GetEmotesForChannelWithUser(ctx, channel, platform, userID)
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

	// Store in cache
	if e.cache != nil {
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
