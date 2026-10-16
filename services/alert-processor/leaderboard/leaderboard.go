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

// Package leaderboard maintains the per-overlay supporter rankings the 'list'
// overlay renders: one Redis sorted set per (overlay, category), score = summed
// amount, member = platform-scoped user key.
package leaderboard

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TTL is the rolling window a board covers. Every write refreshes it, so a
// stream whose gifts keep coming never loses its board, while a board nobody
// touches expires after 30 days instead of accumulating stale rankings forever.
const TTL = 30 * 24 * time.Hour

// Category maps a normalized event type onto its leaderboard category. Only
// amount-bearing events qualify — the categories are exactly bits, gifts and
// super_chat; everything else (follows, shares, raids, plain subs) has no board
// because it carries no spend to rank by.
func Category(eventType string) (string, bool) {
	switch eventType {
	case "bits":
		return "bits", true
	case "mystery_gift", "gift_subscription", "membership_gift", "gift":
		return "gifts", true
	case "super_chat":
		return "super_chat", true
	default:
		return "", false
	}
}

// Member is the sorted-set member for a sender. Platform-scoped because raw
// platform user ids are only unique within their platform.
func Member(platform, userID string) string {
	return platform + ":" + userID
}

// Key is the sorted-set key: overlay:{overlay_id}:leaderboard:{category}.
func Key(overlayID, category string) string {
	return fmt.Sprintf("overlay:%s:leaderboard:%s", overlayID, category)
}

// claimedKey is the once-per-alert claim that guards a board against the
// consumer group's at-least-once redeliveries.
func claimedKey(overlayID, alertID string) string {
	return fmt.Sprintf("overlay:%s:lb-scored:%s", overlayID, alertID)
}

// scoreOnceScript claims the alert's board contribution and scores it as one
// atomic step. ZINCRBY is additive and the consumer group is at-least-once,
// so the same alert can reach a board twice (a redelivery after another
// overlay's step failed, or a crash before the stream ACK) — without the
// SET NX claim the second delivery would permanently double-count the
// amount. Atomicity matters in the other direction too: a claim-then-score
// split across two calls could crash in between and silently under-count the
// amount instead. The claim key's TTL matches the board's, so claims for
// long-expired boards do not accumulate.
var scoreOnceScript = redis.NewScript(`
if redis.call('SETNX', KEYS[1], '1') == 0 then
	return 0
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
redis.call('ZINCRBY', KEYS[2], ARGV[1], ARGV[2])
redis.call('EXPIRE', KEYS[2], ARGV[3])
return 1
`)

// Client writes leaderboard scores to Redis.
type Client struct {
	rdb redis.UniversalClient
}

// New creates a leaderboard client.
func New(rdb redis.UniversalClient) *Client {
	return &Client{rdb: rdb}
}

// RecordAmount adds amount to the member's running score on the board, but
// only the first time this alert is scored on this overlay's board — the
// per-alert claim inside scoreOnceScript absorbs redeliveries. Both the
// board and the claim carry the 30-day rolling TTL.
func (c *Client) RecordAmount(ctx context.Context, overlayID, category, member string, amount float64, alertID string) error {
	key := Key(overlayID, category)
	_, err := scoreOnceScript.Run(ctx, c.rdb,
		[]string{claimedKey(overlayID, alertID), key},
		amount, member, int64(TTL/time.Second)).Result()
	if err != nil {
		return fmt.Errorf("failed to record leaderboard amount for %s: %w", key, err)
	}
	return nil
}
