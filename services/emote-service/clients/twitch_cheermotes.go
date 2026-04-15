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

package clients

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// TwitchCheermoteResponse represents the Twitch Helix cheermotes API response
type TwitchCheermoteResponse struct {
	Data []TwitchCheermotePrefix `json:"data"`
}

// TwitchCheermotePrefix represents a cheermote prefix with its tiers
type TwitchCheermotePrefix struct {
	Prefix string                `json:"prefix"`
	Tiers  []TwitchCheermoteTier `json:"tiers"`
	Type   string                `json:"type"` // "global_first_party", "channel_custom", etc.
}

// TwitchCheermoteTier represents a specific tier of a cheermote
type TwitchCheermoteTier struct {
	MinBits  int                    `json:"min_bits"`
	ID       string                 `json:"id"`
	Color    string                 `json:"color"`
	Images   TwitchCheermoteImages  `json:"images"`
	CanCheer bool                   `json:"can_cheer"`
}

// TwitchCheermoteImages contains image URLs for different themes, types, and scales
type TwitchCheermoteImages struct {
	Dark  TwitchCheermoteTheme `json:"dark"`
	Light TwitchCheermoteTheme `json:"light"`
}

// TwitchCheermoteTheme contains animated and static image sets
type TwitchCheermoteTheme struct {
	Animated TwitchCheermoteScale `json:"animated"`
	Static   TwitchCheermoteScale `json:"static"`
}

// TwitchCheermoteScale contains URLs for different scales
type TwitchCheermoteScale struct {
	One   string `json:"1"`
	Two   string `json:"2"`
	Four  string `json:"4"`
}

// CheermoteData is the simplified format returned to clients
type CheermoteData struct {
	Prefix string
	Tiers  []CheermoteTier
}

// CheermoteTier is the simplified tier format
type CheermoteTier struct {
	MinBits int
	Color   string
	URL     string // Pre-selected animated dark 2x URL
}

// TwitchCheermoteClient fetches Twitch cheermote data
type TwitchCheermoteClient struct {
	helix  *TwitchClient
	logger *zap.Logger
}

// NewTwitchCheermoteClient creates a new Twitch cheermote client
func NewTwitchCheermoteClient(helix *TwitchClient, logger *zap.Logger) *TwitchCheermoteClient {
	return &TwitchCheermoteClient{
		helix:  helix,
		logger: logger.With(zap.String("provider", "twitch-bits")),
	}
}

// FetchCheermotes fetches all cheermotes for a broadcaster
// Returns both global and channel-specific cheermotes
func (c *TwitchCheermoteClient) FetchCheermotes(ctx context.Context, channelID string) ([]CheermoteData, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("channel ID is required")
	}

	// Resolve username to broadcaster ID if needed
	broadcasterID := channelID
	if !isNumeric(channelID) {
		resolved, err := c.helix.GetUserID(ctx, channelID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve twitch user: %w", err)
		}
		broadcasterID = resolved
	}

	// Fetch cheermotes from Helix API
	var resp TwitchCheermoteResponse
	err := c.helix.apiGet(ctx, "/helix/bits/cheermotes", url.Values{
		"broadcaster_id": []string{broadcasterID},
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cheermotes: %w", err)
	}

	// Convert to simplified format
	cheermotes := make([]CheermoteData, 0, len(resp.Data))
	for _, prefix := range resp.Data {
		tiers := make([]CheermoteTier, 0, len(prefix.Tiers))
		for _, tier := range prefix.Tiers {
			// Select animated dark theme, 2x scale
			imageURL := tier.Images.Dark.Animated.Two
			if imageURL == "" {
				// Fallback to static if animated not available
				imageURL = tier.Images.Dark.Static.Two
			}
			if imageURL == "" {
				// Skip tiers without images
				continue
			}

			tiers = append(tiers, CheermoteTier{
				MinBits: tier.MinBits,
				Color:   tier.Color,
				URL:     imageURL,
			})
		}

		// Sort tiers by min_bits ascending for easier lookup
		sort.Slice(tiers, func(i, j int) bool {
			return tiers[i].MinBits < tiers[j].MinBits
		})

		cheermotes = append(cheermotes, CheermoteData{
			Prefix: prefix.Prefix,
			Tiers:  tiers,
		})
	}

	c.logger.Debug("Fetched cheermotes from Twitch Helix",
		zap.String("broadcaster_id", broadcasterID),
		zap.Int("prefix_count", len(cheermotes)),
	)

	return cheermotes, nil
}
