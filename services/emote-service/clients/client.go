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
	"errors"
	"sort"
	"strings"

	"github.com/caesar/all-chat/services/emote-service/models"
	"go.uber.org/zap"
)

// EmoteClient is the interface for fetching emotes from external providers
type EmoteClient interface {
	// FetchEmotes fetches emotes for a given channel
	FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error)

	// Provider returns the provider name
	Provider() string
}

// mergeEmoteSets merges a provider's global and channel emote sets, with the
// channel's emotes taking precedence on code collisions (case-insensitive).
func mergeEmoteSets(global, channel []models.Emote) []models.Emote {
	index := make(map[string]models.Emote)
	for _, emote := range global {
		index[strings.ToLower(emote.Code)] = emote
	}
	for _, emote := range channel {
		index[strings.ToLower(emote.Code)] = emote
	}

	merged := make([]models.Emote, 0, len(index))
	for _, emote := range index {
		merged = append(merged, emote)
	}

	sort.Slice(merged, func(i, j int) bool {
		return strings.ToLower(merged[i].Code) < strings.ToLower(merged[j].Code)
	})

	return merged
}

// mergeChannelWithGlobals combines a channel lookup with a global-set fetch:
// globals are always merged in, a channel miss (ErrNotFound) falls back to the
// global set, and a global failure only degrades when channel emotes exist —
// otherwise the real error propagates instead of caching an empty result.
func mergeChannelWithGlobals(
	logger *zap.Logger,
	provider, channel string,
	channelEmotes []models.Emote, chErr error,
	globalEmotes []models.Emote, gErr error,
) ([]models.Emote, error) {
	if gErr != nil {
		// Global emotes are a bonus, not a requirement — failing to fetch them
		// must not lose channel emotes. But with nothing else to return, the
		// fetch either missed (ErrNotFound) or returned no emotes, so propagate
		// the real error instead of caching an empty result.
		if len(channelEmotes) == 0 {
			return nil, gErr
		}
		logger.Warn("Failed to fetch "+provider+" global emotes, returning channel emotes only",
			zap.String("channel", channel),
			zap.Error(gErr))
		return channelEmotes, nil
	}

	if errors.Is(chErr, ErrNotFound) {
		return globalEmotes, nil
	}

	merged := mergeEmoteSets(globalEmotes, channelEmotes)

	logger.Debug("Fetched "+provider+" emotes",
		zap.String("channel", channel),
		zap.Int("channel_emotes", len(channelEmotes)),
		zap.Int("global_emotes", len(globalEmotes)),
		zap.Int("total", len(merged)))

	return merged, nil
}
