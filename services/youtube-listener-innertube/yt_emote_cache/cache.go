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
