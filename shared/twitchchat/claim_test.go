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

package twitchchat

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T, ttl time.Duration) (*miniredis.Miniredis, *ClaimStore) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return mr, NewClaimStoreWithTTL(rc, ttl)
}

func TestClaimKey_LowercasesAndPrefixes(t *testing.T) {
	if got, want := ClaimKey("CaesarLP"), "eventsub:chat:owner:caesarlp"; got != want {
		t.Fatalf("ClaimKey = %q, want %q", got, want)
	}
}

func TestClaim_SetsKeyWithTTL(t *testing.T) {
	mr, store := newTestStore(t, 90*time.Second)
	ctx := context.Background()

	if err := store.Claim(ctx, "CaesarLP", "67241623"); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// Stored under the lower-cased key with the broadcaster id as value.
	got, err := mr.Get("eventsub:chat:owner:caesarlp")
	if err != nil {
		t.Fatalf("expected key to exist: %v", err)
	}
	if got != "67241623" {
		t.Fatalf("value = %q, want %q", got, "67241623")
	}

	// TTL is set (not persistent).
	if ttl := mr.TTL("eventsub:chat:owner:caesarlp"); ttl <= 0 || ttl > 90*time.Second {
		t.Fatalf("TTL = %v, want (0, 90s]", ttl)
	}
}

func TestClaim_ExpiresAfterTTL(t *testing.T) {
	mr, store := newTestStore(t, 90*time.Second)
	ctx := context.Background()

	if err := store.Claim(ctx, "quietchannel", "1"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if !mr.Exists("eventsub:chat:owner:quietchannel") {
		t.Fatal("claim should exist immediately after Claim")
	}

	// No refresh within the TTL → claim lapses → IRC resumes the channel.
	mr.FastForward(91 * time.Second)
	if mr.Exists("eventsub:chat:owner:quietchannel") {
		t.Fatal("claim should have expired after TTL with no refresh")
	}
}

func TestClaim_RefreshExtendsTTL(t *testing.T) {
	mr, store := newTestStore(t, 90*time.Second)
	ctx := context.Background()

	_ = store.Claim(ctx, "active", "1")
	mr.FastForward(60 * time.Second) // 30s of TTL left
	_ = store.Claim(ctx, "active", "1")
	mr.FastForward(60 * time.Second) // would have expired without the refresh

	if !mr.Exists("eventsub:chat:owner:active") {
		t.Fatal("refresh should have extended the TTL; claim must still exist")
	}
}

func TestRelease_DeletesClaim(t *testing.T) {
	mr, store := newTestStore(t, 90*time.Second)
	ctx := context.Background()

	_ = store.Claim(ctx, "Revoked", "1")
	if err := store.Release(ctx, "revoked"); err != nil { // case-insensitive
		t.Fatalf("Release: %v", err)
	}
	if mr.Exists("eventsub:chat:owner:revoked") {
		t.Fatal("Release should have deleted the claim")
	}
}

func TestRelease_MissingClaimIsNoError(t *testing.T) {
	_, store := newTestStore(t, 90*time.Second)
	if err := store.Release(context.Background(), "never-claimed"); err != nil {
		t.Fatalf("Release of missing claim should be a no-op, got %v", err)
	}
}

func TestClaimedLogins_ReturnsLowercasedSet(t *testing.T) {
	_, store := newTestStore(t, 90*time.Second)
	ctx := context.Background()

	_ = store.Claim(ctx, "CaesarLP", "1")
	_ = store.Claim(ctx, "another", "2")
	_ = store.Claim(ctx, "Third", "3")
	_ = store.Release(ctx, "another")

	claimed, err := store.ClaimedLogins(ctx)
	if err != nil {
		t.Fatalf("ClaimedLogins: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("len(claimed) = %d, want 2 (%v)", len(claimed), claimed)
	}
	if _, ok := claimed["caesarlp"]; !ok {
		t.Errorf("expected lower-cased 'caesarlp' in claimed set: %v", claimed)
	}
	if _, ok := claimed["third"]; !ok {
		t.Errorf("expected lower-cased 'third' in claimed set: %v", claimed)
	}
	if _, ok := claimed["another"]; ok {
		t.Errorf("released login 'another' should not be claimed: %v", claimed)
	}
}

func TestClaimedLogins_EmptyWhenNoClaims(t *testing.T) {
	_, store := newTestStore(t, 90*time.Second)
	claimed, err := store.ClaimedLogins(context.Background())
	if err != nil {
		t.Fatalf("ClaimedLogins: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("want empty set, got %v", claimed)
	}
}

func TestNewClaimStoreWithTTL_NonPositiveFallsBackToDefault(t *testing.T) {
	// No Redis I/O needed — only the TTL guard is under test.
	store := NewClaimStoreWithTTL(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), 0)
	if store.TTL() != DefaultClaimTTL {
		t.Fatalf("TTL = %v, want default %v", store.TTL(), DefaultClaimTTL)
	}
	if neg := NewClaimStoreWithTTL(nil, -5*time.Second); neg.TTL() != DefaultClaimTTL {
		t.Fatalf("negative TTL = %v, want default %v", neg.TTL(), DefaultClaimTTL)
	}
}
