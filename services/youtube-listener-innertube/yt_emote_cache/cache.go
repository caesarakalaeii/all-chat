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

package yt_emote_cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const EmoteTTL = 24 * time.Hour

// EmoteEntry represents a custom YouTube emote for caching.
// Must match the EmoteEntry struct in innertube/parser.go (duplicated to avoid cross-package coupling).
type EmoteEntry struct {
	Code string `json:"code"`
	URL  string `json:"url"`
	ID   string `json:"id"`
}

// CacheYTEmotes writes each emote to Redis using the key pattern yt:emote:{channelID}:{emojiID}.
// Uses a 500ms timeout derived from ctx. Errors are returned but callers should log and continue.
func CacheYTEmotes(ctx context.Context, rdb *redis.Client, channelID string, emotes []EmoteEntry) error {
	if len(emotes) == 0 {
		return nil
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	for _, e := range emotes {
		if e.ID == "" {
			continue
		}
		key := fmt.Sprintf("yt:emote:%s:%s", channelID, e.ID)
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		// SET (not SETNX) to refresh TTL on each seen emote
		if err := rdb.Set(cacheCtx, key, data, EmoteTTL).Err(); err != nil {
			return fmt.Errorf("cache emote %s: %w", key, err)
		}
	}
	return nil
}
