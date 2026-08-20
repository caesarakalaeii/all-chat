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
//
// Cross-pod dedup:
//   - With multiple api-gateway replicas, every pod that has the Pub/Sub
//     subscription open will see the same message and try to buffer it.
//   - AddOnce uses a stable per-message marker (SETNX) so the ZADD only runs
//     on the first pod to see the message; the rest no-op. This keeps the
//     buffer free of cross-pod duplicates without coordination.
type ChatReplayBuffer interface {
	// Add stores a serialised WSMessage envelope keyed by ms-precision timestamp.
	// Caller passes the raw JSON bytes that are sent on the wire.
	// Use AddOnce if you have a stable message ID that should suppress duplicates.
	Add(ctx context.Context, overlayID string, payload []byte, ts time.Time) error

	// AddOnce is Add gated by a per-message SETNX marker. Returns (true, nil)
	// if the message was newly buffered, (false, nil) if a prior pod already
	// buffered the same messageID for the same overlay within the TTL window.
	// messageID must be a stable, globally unique identifier (e.g. the unified
	// message UUID published by message-processor).
	AddOnce(ctx context.Context, overlayID, messageID string, payload []byte, ts time.Time) (bool, error)

	// GetSince returns all buffered envelopes with timestamp > sinceMs, plus
	// whether the requested watermark predates what the buffer still holds.
	// Pass 0 to fetch the entire buffer.
	GetSince(ctx context.Context, overlayID string, sinceMs int64) (ChatReplay, error)
}

// ChatReplay is the result of a GetSince query: the envelopes to send, and
// whether the answer is known to be incomplete.
//
// Truncated exists because the buffer is bounded twice over — by TTL (a sliding
// window, default 5 minutes) and by MaxEntries (default 500, oldest dropped on
// overflow). A client away for ten minutes, or one whose overlay took 500+
// messages during a thirty-second gap, would otherwise receive a silently
// short replay and believe it was caught up. Reporting the shortfall is the
// whole point: it is deliberately *not* fixed by widening the window.
type ChatReplay struct {
	// Messages are the buffered envelopes with timestamp > sinceMs, in
	// chronological order.
	Messages [][]byte

	// Truncated is true when sinceMs predates the oldest entry the buffer
	// still holds — meaning messages between the client's watermark and the
	// start of the buffer existed and are gone. It reaches clients as
	// `replay_truncated` on the connected frame (models.ConnectedData /
	// models.ViewerConnectedData).
	//
	// It is always false for sinceMs <= 0 ("replay everything"): a caller
	// asking for the whole buffer is making no claim about what it already
	// saw, so there is no gap to report against. It is also false for an
	// empty buffer, where there is no oldest entry to be older than.
	Truncated bool
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

// chatSeenKey returns the per-message SETNX marker key used by AddOnce.
// One key per (overlay, message) pair, expired by the buffer's TTL.
func chatSeenKey(overlayID, messageID string) string {
	return fmt.Sprintf("replay:chat:%s:seen:%s", overlayID, messageID)
}

// AddOnce wraps Add with a SETNX marker keyed on messageID. If the marker
// already exists (another pod buffered it first), AddOnce returns (false, nil)
// without modifying the sorted set. Otherwise the marker is set with the same
// TTL as the buffer, the message is appended, and (true, nil) is returned.
//
// Failure to SETNX is treated as "first time, proceed" — preferring the rare
// duplicate over the worse failure mode of dropping a message under Redis
// hiccup. The frontend's render-time ID dedup is the final safety net.
func (b *RedisChatReplayBuffer) AddOnce(ctx context.Context, overlayID, messageID string, payload []byte, ts time.Time) (bool, error) {
	if messageID == "" {
		// No stable ID — fall back to unconditional Add.
		return true, b.Add(ctx, overlayID, payload, ts)
	}

	seenKey := chatSeenKey(overlayID, messageID)
	ok, err := b.client.SetNX(ctx, seenKey, "1", b.ttl).Result()
	if err != nil {
		// Best-effort dedup; on Redis error proceed as if first-seen.
		return true, b.Add(ctx, overlayID, payload, ts)
	}
	if !ok {
		// Another pod already buffered this message.
		return false, nil
	}
	return true, b.Add(ctx, overlayID, payload, ts)
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

// GetSince returns all members with score > sinceMs, in chronological order,
// and reports whether sinceMs predates the oldest entry still buffered.
// Strips the disambiguator suffix added in Add before returning to the caller.
func (b *RedisChatReplayBuffer) GetSince(ctx context.Context, overlayID string, sinceMs int64) (ChatReplay, error) {
	key := chatKey(overlayID)

	min := "-inf"
	if sinceMs > 0 {
		// Exclusive lower bound so passing the last-seen timestamp does not
		// re-deliver the message at that exact ms.
		min = fmt.Sprintf("(%d", sinceMs)
	}

	truncated := b.isTruncated(ctx, key, sinceMs)

	results, err := b.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: min,
		Max: "+inf",
	}).Result()
	if err == redis.Nil {
		return ChatReplay{}, nil
	}
	if err != nil {
		return ChatReplay{}, fmt.Errorf("failed to query chat replay buffer: %w", err)
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
	return ChatReplay{Messages: out, Truncated: truncated}, nil
}

// isTruncated reports whether a client asking for messages after sinceMs is
// asking for something the buffer has already evicted.
//
// The check is one ZRANGE of a single element: read the lowest score in the
// sorted set and compare. If the oldest entry we still hold is *newer* than the
// client's watermark, everything between the two was dropped by the TTL or the
// MaxEntries cap and the replay we are about to send is short.
//
// A watermark of 0 or less means "replay everything" — the caller is claiming
// nothing about what it already saw, so nothing can be missing relative to it.
// An empty buffer is likewise not truncated: there is no oldest entry.
//
// A Redis failure here is not fatal to the replay. Falling back to "not
// truncated" matches the pre-existing behaviour exactly, and the alternative —
// failing a reconnect because a diagnostic hint could not be computed — trades
// a missing warning for actual message loss.
func (b *RedisChatReplayBuffer) isTruncated(ctx context.Context, key string, sinceMs int64) bool {
	if sinceMs <= 0 {
		return false
	}

	oldest, err := b.client.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil || len(oldest) == 0 {
		// redis.Nil, a transport error, or an empty buffer: no gap to report.
		return false
	}

	// Scores are ms-precision UnixMilli values written by Add.
	return int64(oldest[0].Score) > sinceMs
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
