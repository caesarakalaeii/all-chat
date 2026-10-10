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

// Package youtubeclaim holds the shared contract for official-API channel ownership (ADR-0065).
// A claim means "the Data API listener is serving this channel right now"; that listener writes
// and refreshes it for every eligible, ownership-verified channel, and the innertube listener
// skips claimed channels. Both listeners import this package so the key format and TTL cannot
// drift between producer and consumer. Mirrors shared/twitchchat (ADR-0015).
package youtubeclaim

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// ClaimKeyPrefix namespaces per-channel official-API claim keys in Redis.
	ClaimKeyPrefix = "youtube:official:claim:"

	// DefaultClaimTTL is how long a claim survives without a refresh. The Data API listener
	// refreshes every 60s, so three missed rounds (pod gone, Redis blip, lost leadership)
	// hand the channel back to innertube.
	DefaultClaimTTL = 3 * time.Minute

	// claimScanCount is the COUNT hint for SCAN; channel counts are small, so one batch suffices.
	claimScanCount = 256
)

// ClaimKey returns the Redis key for a channel's claim. Unlike Twitch logins, YouTube channel ids
// are case-sensitive, so the id is used verbatim.
func ClaimKey(channelID string) string {
	return ClaimKeyPrefix + channelID
}

// ClaimStore reads and writes official-API claims in Redis. It is safe for concurrent use.
type ClaimStore struct {
	rc  *redis.Client
	ttl time.Duration
}

// NewClaimStore creates a ClaimStore with the default TTL.
func NewClaimStore(rc *redis.Client) *ClaimStore {
	return NewClaimStoreWithTTL(rc, DefaultClaimTTL)
}

// NewClaimStoreWithTTL creates a ClaimStore with a custom claim TTL. A non-positive ttl falls back
// to DefaultClaimTTL so a misconfiguration can never set an immediately-expiring claim.
func NewClaimStoreWithTTL(rc *redis.Client, ttl time.Duration) *ClaimStore {
	if ttl <= 0 {
		ttl = DefaultClaimTTL
	}
	return &ClaimStore{rc: rc, ttl: ttl}
}

// TTL returns the configured claim TTL.
func (s *ClaimStore) TTL() time.Duration { return s.ttl }

// Claim creates or refreshes the claim for channelID, resetting its TTL. value is stored for
// operator visibility (the verified owner's user id) and is not interpreted.
func (s *ClaimStore) Claim(ctx context.Context, channelID, value string) error {
	return s.rc.Set(ctx, ClaimKey(channelID), value, s.ttl).Err()
}

// Release deletes the claim for channelID so innertube resumes the channel on its next sync
// instead of waiting out the TTL.
func (s *ClaimStore) Release(ctx context.Context, channelID string) error {
	return s.rc.Del(ctx, ClaimKey(channelID)).Err()
}

// IsClaimed reports whether channelID currently holds a live claim. Single-key EXISTS, for paths
// that ask about one channel and must not pay for a full SCAN.
func (s *ClaimStore) IsClaimed(ctx context.Context, channelID string) (bool, error) {
	n, err := s.rc.Exists(ctx, ClaimKey(channelID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClaimedChannels returns the set of channel ids that currently hold a live claim. Uses SCAN (not
// KEYS) so it is safe to call on the periodic sync path.
func (s *ClaimStore) ClaimedChannels(ctx context.Context) (map[string]struct{}, error) {
	claimed := make(map[string]struct{})
	iter := s.rc.Scan(ctx, 0, ClaimKeyPrefix+"*", claimScanCount).Iterator()
	for iter.Next(ctx) {
		channelID := strings.TrimPrefix(iter.Val(), ClaimKeyPrefix)
		if channelID != "" {
			claimed[channelID] = struct{}{}
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return claimed, nil
}
