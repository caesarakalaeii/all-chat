package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

// EmoteServiceClient is an interface for calling the Emote Service
type EmoteServiceClient interface {
	GetEmotesForChannel(ctx context.Context, channel string) ([]EmoteServiceEmote, error)
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
	url := fmt.Sprintf("%s/emotes/channel/%s", c.baseURL, channel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	client EmoteServiceClient
	logger *zap.Logger
}

// NewEnricher creates a new emote enricher
func NewEnricher(client EmoteServiceClient, logger *zap.Logger) *Enricher {
	return &Enricher{
		client: client,
		logger: logger,
	}
}

// Enrich adds third-party emotes (7TV, BTTV, FFZ) to the message
func (e *Enricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	// Fetch emotes for the channel
	thirdPartyEmotes, err := e.client.GetEmotesForChannel(ctx, msg.ChannelID)
	if err != nil {
		// Don't fail the message if emote enrichment fails
		e.logger.Warn("Failed to fetch emotes, skipping enrichment",
			zap.String("channel", msg.ChannelID),
			zap.Error(err),
		)
		return nil
	}

	// Build a map of emote code -> emote for quick lookup
	emoteMap := make(map[string]EmoteServiceEmote)
	for _, emote := range thirdPartyEmotes {
		emoteMap[emote.Code] = emote
	}

	// Tokenize message text and find matching emotes
	words := strings.Fields(msg.Message.Text)
	for i, word := range words {
		if emote, found := emoteMap[word]; found {
			// Calculate positions in the original text
			position := e.findWordPosition(msg.Message.Text, word, i)
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
