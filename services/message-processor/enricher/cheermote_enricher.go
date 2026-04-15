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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CheermoteServiceClient is an interface for calling the Emote Service cheermotes endpoint
type CheermoteServiceClient interface {
	GetCheermotesForChannel(ctx context.Context, channelID string) ([]CheermoteData, error)
}

// CheermoteData represents cheermote data for a specific prefix
type CheermoteData struct {
	Prefix string          `json:"prefix"`
	Tiers  []CheermoteTier `json:"tiers"`
}

// CheermoteTier represents a specific tier of a cheermote
type CheermoteTier struct {
	MinBits int    `json:"min_bits"`
	Color   string `json:"color"`
	URL     string `json:"url"` // Pre-selected animated dark 2x URL
}

// CheermoteServiceResponse is the response from Emote Service cheermotes endpoint
type CheermoteServiceResponse struct {
	ChannelID  string           `json:"channel_id"`
	Cheermotes []CheermoteData `json:"cheermotes"`
}

// HTTPCheermoteClient implements CheermoteServiceClient using HTTP
type HTTPCheermoteClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// NewHTTPCheermoteClient creates a new HTTP client for Cheermotes Service
func NewHTTPCheermoteClient(baseURL string, logger *zap.Logger) *HTTPCheermoteClient {
	return &HTTPCheermoteClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// GetCheermotesForChannel fetches all cheermotes for a channel from the Emote Service
func (c *HTTPCheermoteClient) GetCheermotesForChannel(ctx context.Context, channelID string) ([]CheermoteData, error) {
	escapedChannel := url.PathEscape(channelID)
	endpoint, err := url.JoinPath(c.baseURL, "emotes", "cheermotes", escapedChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to build cheermote service url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call cheermote service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cheermote service returned status %d", resp.StatusCode)
	}

	var cheermoteResp CheermoteServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&cheermoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return cheermoteResp.Cheermotes, nil
}

// CheermoteEnricher enriches Twitch messages with cheermote emotes
type CheermoteEnricher struct {
	client       CheermoteServiceClient
	redisClient  *redis.Client
	logger       *zap.Logger
	cacheTTL     time.Duration
	// Regex pattern to match cheermotes: word boundary + prefix + digits
	cheermotePattern *regexp.Regexp
}

// NewCheermoteEnricher creates a new cheermote enricher
func NewCheermoteEnricher(client CheermoteServiceClient, redisClient *redis.Client, logger *zap.Logger) *CheermoteEnricher {
	return &CheermoteEnricher{
		client:           client,
		redisClient:      redisClient,
		logger:           logger,
		cacheTTL:         1 * time.Hour, // Cheermotes rarely change
		cheermotePattern: regexp.MustCompile(`\b([A-Za-z]+)(\d+)\b`),
	}
}

// Enrich adds cheermote emotes to Twitch messages that contain bits
func (e *CheermoteEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	// Only process Twitch messages
	if msg.Platform != "twitch" {
		return nil
	}

	// Check if message has bits
	bitsInterface, hasBits := msg.Metadata["bits"]
	if !hasBits {
		return nil
	}

	bits, ok := bitsInterface.(int)
	if !ok || bits == 0 {
		return nil
	}

	// Get channel identifier (prefer room_id for Twitch)
	channelIdentifier := msg.ChannelID
	if roomID, ok := msg.Metadata["twitch_room_id"]; ok {
		switch v := roomID.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				channelIdentifier = v
			}
		}
	}

	// Fetch cheermote data for the channel
	cheermotes, err := e.fetchCheermotes(ctx, channelIdentifier)
	if err != nil {
		// Don't fail the message if cheermote enrichment fails
		e.logger.Warn("Failed to fetch cheermotes, skipping enrichment",
			zap.String("channel", msg.ChannelID),
			zap.Error(err),
		)
		return nil
	}

	// Build a map of lowercase prefix -> cheermote data
	cheermoteMap := make(map[string]CheermoteData)
	for _, cheermote := range cheermotes {
		cheermoteMap[strings.ToLower(cheermote.Prefix)] = cheermote
	}

	// Find all cheermote patterns in message text
	matches := e.cheermotePattern.FindAllStringSubmatchIndex(msg.Message.Text, -1)

	for _, match := range matches {
		// match[0], match[1]: full match start/end
		// match[2], match[3]: prefix start/end
		// match[4], match[5]: digits start/end

		if len(match) < 6 {
			continue
		}

		prefix := msg.Message.Text[match[2]:match[3]]
		bitsStr := msg.Message.Text[match[4]:match[5]]

		// Parse bits amount
		bitsAmount, err := strconv.Atoi(bitsStr)
		if err != nil || bitsAmount <= 0 {
			continue
		}

		// Look up cheermote (case-insensitive)
		cheermote, found := cheermoteMap[strings.ToLower(prefix)]
		if !found {
			continue
		}

		// Find appropriate tier for this bits amount
		tier := e.findTier(cheermote.Tiers, bitsAmount)
		if tier == nil {
			continue
		}

		// Add cheermote to emotes array
		msg.Message.Emotes = append(msg.Message.Emotes, models.Emote{
			Code:      fmt.Sprintf("%s%d", prefix, bitsAmount), // e.g., "Cheer100"
			Provider:  "twitch-bits",
			URL:       tier.URL,
			Positions: [][]int{{match[0], match[1] - 1}}, // Full match positions
		})

		e.logger.Debug("Added cheermote to message",
			zap.String("channel", msg.ChannelID),
			zap.String("prefix", prefix),
			zap.Int("bits", bitsAmount),
			zap.Int("tier_min_bits", tier.MinBits),
		)
	}

	return nil
}

// findTier finds the highest tier that matches the bits amount
// Example: 250 bits with tiers [1, 100, 500, 1000] → returns tier 100
func (e *CheermoteEnricher) findTier(tiers []CheermoteTier, bits int) *CheermoteTier {
	var selectedTier *CheermoteTier

	for i := range tiers {
		tier := &tiers[i]
		if bits >= tier.MinBits {
			// Keep checking for higher tiers
			if selectedTier == nil || tier.MinBits > selectedTier.MinBits {
				selectedTier = tier
			}
		}
	}

	return selectedTier
}

// fetchCheermotes fetches cheermote data with caching
func (e *CheermoteEnricher) fetchCheermotes(ctx context.Context, channelID string) ([]CheermoteData, error) {
	cacheKey := fmt.Sprintf("mp:cheermotes:v1:%s", channelID)

	// Try cache first
	if e.redisClient != nil {
		data, err := e.redisClient.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var cached []CheermoteData
			if err := json.Unmarshal(data, &cached); err == nil {
				e.logger.Debug("Cheermote cache hit",
					zap.String("channel", channelID),
					zap.Int("count", len(cached)),
				)
				return cached, nil
			}
		} else if err != redis.Nil {
			e.logger.Warn("Cheermote cache error",
				zap.String("channel", channelID),
				zap.Error(err),
			)
		}
	}

	// Cache miss - fetch from service
	cheermotes, err := e.client.GetCheermotesForChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if e.redisClient != nil {
		if data, err := json.Marshal(cheermotes); err == nil {
			if err := e.redisClient.Set(ctx, cacheKey, data, e.cacheTTL).Err(); err != nil {
				e.logger.Warn("Failed to populate cheermote cache",
					zap.String("channel", channelID),
					zap.Error(err),
				)
			} else {
				e.logger.Debug("Cheermote cache populated",
					zap.String("channel", channelID),
					zap.Int("count", len(cheermotes)),
				)
			}
		}
	}

	return cheermotes, nil
}
