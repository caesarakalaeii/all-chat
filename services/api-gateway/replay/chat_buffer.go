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

package replay

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ChatReplayBuffer stores recent chat-message WebSocket envelopes for replay
// when a client reconnects after a brief disconnect. The buffer holds the
// fully-formed WSMessage JSON so reconnecting clients receive exactly the
// same bytes they would have received live.
//
// Cardinality concerns:
//   - One sorted set per overlay (replay:chat:{overlay_id}).
//   - Bounded by both TTL (sliding window) and MaxEntries (drop oldest on overflow).
//   - Buffer is only populated for overlays that currently have a Pub/Sub
//     subscription, so dormant overlays do not accumulate Redis memory.
type ChatReplayBuffer interface {
	// Add stores a serialised WSMessage envelope keyed by ms-precision timestamp.
	// Caller passes the raw JSON bytes that are sent on the wire.
	Add(ctx context.Context, overlayID string, payload []byte, ts time.Time) error

	// GetSince returns all buffered envelopes with timestamp > sinceMs.
	// Pass 0 to fetch the entire buffer.
	GetSince(ctx context.Context, overlayID string, sinceMs int64) ([][]byte, error)
}

// RedisChatReplayBuffer is the production implementation backed by Redis.
type RedisChatReplayBuffer struct {
	client     *redis.Client
	ttl        time.Duration // sliding window the buffer covers
	maxEntries int           // hard cap per overlay to bound memory
}

// NewRedisChatReplayBuffer creates a chat replay buffer with the given TTL
// (sliding window) and per-overlay entry cap.
//
// Recommended values for hiccup recovery:
//   - ttl: 5 minutes (matches youtube-listener demand-stop debounce)
//   - maxEntries: 500 (~100 msg/s burst × 5s; cheaper than time-based pruning)
func NewRedisChatReplayBuffer(client *redis.Client, ttl time.Duration, maxEntries int) *RedisChatReplayBuffer {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	return &RedisChatReplayBuffer{
		client:     client,
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// chatKey returns the Redis key for a given overlay's chat buffer.
func chatKey(overlayID string) string {
	return fmt.Sprintf("replay:chat:%s", overlayID)
}

// Add appends a payload to the buffer. The score is the millisecond-precision
// timestamp; if two messages share the same ms we append a small disambiguator
// suffix to the member so neither overwrites the other.
func (b *RedisChatReplayBuffer) Add(ctx context.Context, overlayID string, payload []byte, ts time.Time) error {
	key := chatKey(overlayID)
	score := float64(ts.UnixMilli())

	// Append nanosecond suffix so duplicate-ms messages do not deduplicate via
	// the ZADD member-uniqueness rule. The suffix is delimited by a NUL byte
	// so it cannot collide with legitimate JSON content.
	member := make([]byte, 0, len(payload)+16)
	member = append(member, payload...)
	member = append(member, 0x00)
	member = strconv.AppendInt(member, int64(ts.Nanosecond()), 10)

	pipe := b.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: string(member)})
	// Trim oldest entries beyond MaxEntries (ZREMRANGEBYRANK negative-indexed).
	// Keep the most recent maxEntries items: remove rank [0, -maxEntries-1].
	pipe.ZRemRangeByRank(ctx, key, 0, int64(-b.maxEntries-1))
	pipe.Expire(ctx, key, b.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// GetSince returns all members with score > sinceMs, in chronological order.
// Strips the disambiguator suffix added in Add before returning to the caller.
func (b *RedisChatReplayBuffer) GetSince(ctx context.Context, overlayID string, sinceMs int64) ([][]byte, error) {
	key := chatKey(overlayID)

	min := "-inf"
	if sinceMs > 0 {
		// Exclusive lower bound so passing the last-seen timestamp does not
		// re-deliver the message at that exact ms.
		min = fmt.Sprintf("(%d", sinceMs)
	}

	results, err := b.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: min,
		Max: "+inf",
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query chat replay buffer: %w", err)
	}

	out := make([][]byte, 0, len(results))
	for _, raw := range results {
		// Strip suffix appended in Add (everything from the NUL byte onward).
		if idx := indexOfByte(raw, 0x00); idx >= 0 {
			out = append(out, []byte(raw[:idx]))
		} else {
			// Legacy entry without suffix (e.g. from older versions or tests).
			out = append(out, []byte(raw))
		}
	}
	return out, nil
}

// indexOfByte returns the index of the first occurrence of b in s, or -1.
// Avoids importing strings/bytes for a one-line helper used in a hot path.
func indexOfByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
