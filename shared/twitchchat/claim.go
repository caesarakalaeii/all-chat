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

// Package twitchchat holds the shared contract for the dynamic IRC↔EventSub chat-ownership
// partition (ADR-0015). A chat-ownership claim means "the EventSub listener is delivering this
// channel's chat right now"; the EventSub webhook handler writes/refreshes it on delivered chat
// and the IRC listener excludes claimed channels from its desired set. Both listeners import this
// package so the key format and TTL cannot drift between producer and consumer.
package twitchchat

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// ClaimKeyPrefix namespaces per-channel chat-ownership claim keys in Redis.
	// A live key at ClaimKey(login) means EventSub currently owns that channel's chat.
	ClaimKeyPrefix = "eventsub:chat:owner:"

	// DefaultClaimTTL is how long a claim survives without a refresh. The EventSub handler
	// refreshes on delivered chat (throttled to ClaimRefreshInterval), so a channel with at
	// least one chat message per TTL stays claimed (and off IRC). When EventSub stops
	// delivering — revocation, verification failure, scope gap, pod/total outage — the claim
	// lapses within this window and the IRC listener resumes the channel. It therefore also
	// bounds the worst-case fallback delay in a total-EventSub-outage scenario. Overridable per
	// deployment via NewClaimStoreWithTTL (EVENTSUB_CHAT_CLAIM_TTL).
	DefaultClaimTTL = 5 * time.Minute

	// ClaimRefreshInterval throttles how often delivered chat refreshes a claim, bounding Redis
	// writes to one per channel per interval regardless of chat volume. Must be comfortably less
	// than the claim TTL so a steadily-active channel never lets its claim lapse.
	ClaimRefreshInterval = 60 * time.Second

	// claimScanCount is the COUNT hint for SCAN; channel counts are small, so one batch suffices.
	claimScanCount = 256
)

// ClaimKey returns the Redis key for a channel's chat-ownership claim. The login is lower-cased so
// the EventSub producer (which sees the broadcaster login from the event) and the IRC consumer
// (which keys on overlay_chat_sources.channel_id) agree regardless of stored casing.
func ClaimKey(login string) string {
	return ClaimKeyPrefix + strings.ToLower(login)
}

// ClaimStore reads and writes chat-ownership claims in Redis. It is safe for concurrent use.
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

// Claim creates or refreshes the chat-ownership claim for login, resetting its TTL. value is stored
// for operator visibility (e.g. the broadcaster id) and is not interpreted. Call this on every
// delivered chat message (throttled by ClaimRefreshInterval at the caller).
func (s *ClaimStore) Claim(ctx context.Context, login, value string) error {
	return s.rc.Set(ctx, ClaimKey(login), value, s.ttl).Err()
}

// Release deletes the chat-ownership claim for login. Best-effort early teardown (e.g. on
// revocation) so IRC resumes the channel promptly; the TTL would expire the claim regardless.
func (s *ClaimStore) Release(ctx context.Context, login string) error {
	return s.rc.Del(ctx, ClaimKey(login)).Err()
}

// ClaimedLogins returns the set of lower-cased logins that currently hold a live chat-ownership
// claim. The IRC listener excludes these from its desired channel set. Uses SCAN (not KEYS) so it
// is safe to call on the periodic sync path.
func (s *ClaimStore) ClaimedLogins(ctx context.Context) (map[string]struct{}, error) {
	claimed := make(map[string]struct{})
	iter := s.rc.Scan(ctx, 0, ClaimKeyPrefix+"*", claimScanCount).Iterator()
	for iter.Next(ctx) {
		login := strings.TrimPrefix(iter.Val(), ClaimKeyPrefix)
		if login != "" {
			claimed[login] = struct{}{}
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return claimed, nil
}
