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

package youtubeclaim

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

func TestClaimKey_PreservesCase(t *testing.T) {
	// YouTube channel ids are case-sensitive base64; lower-casing would merge distinct channels.
	if got, want := ClaimKey("UCabcDEF_123"), "youtube:official:claim:UCabcDEF_123"; got != want {
		t.Fatalf("ClaimKey = %q, want %q", got, want)
	}
	if ClaimKey("UCabc") == ClaimKey("UCABC") {
		t.Fatal("ClaimKey must not fold case")
	}
}

func TestClaim_SetsTTL(t *testing.T) {
	mr, store := newTestStore(t, 0)
	if store.TTL() != DefaultClaimTTL || DefaultClaimTTL != 3*time.Minute {
		t.Fatalf("TTL = %v, DefaultClaimTTL = %v, want 3m", store.TTL(), DefaultClaimTTL)
	}
	if err := store.Claim(context.Background(), "UCowner", "user-1"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	got, err := mr.Get("youtube:official:claim:UCowner")
	if err != nil {
		t.Fatalf("expected key to exist: %v", err)
	}
	if got != "user-1" {
		t.Fatalf("value = %q, want %q", got, "user-1")
	}
	if ttl := mr.TTL("youtube:official:claim:UCowner"); ttl <= 0 || ttl > 3*time.Minute {
		t.Fatalf("TTL = %v, want (0, 3m]", ttl)
	}
}

func TestClaim_Expires(t *testing.T) {
	mr, store := newTestStore(t, 0)
	_ = store.Claim(context.Background(), "UCquiet", "u")
	mr.FastForward(3*time.Minute + time.Second)
	if mr.Exists("youtube:official:claim:UCquiet") {
		t.Fatal("claim should have expired after TTL with no refresh")
	}
}

func TestClaim_RefreshExtends(t *testing.T) {
	mr, store := newTestStore(t, 0)
	ctx := context.Background()
	_ = store.Claim(ctx, "UCactive", "u")
	mr.FastForward(2 * time.Minute)
	_ = store.Claim(ctx, "UCactive", "u")
	mr.FastForward(2 * time.Minute)
	if !mr.Exists("youtube:official:claim:UCactive") {
		t.Fatal("refresh should have extended the TTL; claim must still exist")
	}
}

func TestRelease_Deletes(t *testing.T) {
	mr, store := newTestStore(t, 0)
	ctx := context.Background()
	_ = store.Claim(ctx, "UCgone", "u")
	if err := store.Release(ctx, "UCgone"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if mr.Exists("youtube:official:claim:UCgone") {
		t.Fatal("Release should have deleted the claim")
	}
	if err := store.Release(ctx, "UCnever"); err != nil {
		t.Fatalf("Release of a missing claim: %v", err)
	}
}

func TestClaimedChannels_Lists(t *testing.T) {
	mr, store := newTestStore(t, 0)
	ctx := context.Background()
	_ = store.Claim(ctx, "UCone", "u1")
	_ = store.Claim(ctx, "UCTwo", "u2")
	_ = mr.Set("eventsub:chat:owner:unrelated", "x")

	got, err := store.ClaimedChannels(ctx)
	if err != nil {
		t.Fatalf("ClaimedChannels: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v, want exactly UCone and UCTwo", got)
	}
	for _, ch := range []string{"UCone", "UCTwo"} {
		if _, ok := got[ch]; !ok {
			t.Fatalf("missing %q in %v", ch, got)
		}
	}
}

func TestIsClaimed_ReportsLiveClaim(t *testing.T) {
	mr, store := newTestStore(t, 0)
	ctx := context.Background()
	_ = store.Claim(ctx, "UClive", "u")
	_ = store.Claim(ctx, "UCstale", "u")
	mr.FastForward(2 * time.Minute)
	_ = store.Claim(ctx, "UClive", "u")
	mr.FastForward(2 * time.Minute)

	for ch, want := range map[string]bool{"UClive": true, "UCstale": false, "UCabsent": false} {
		got, err := store.IsClaimed(ctx, ch)
		if err != nil {
			t.Fatalf("IsClaimed(%q): %v", ch, err)
		}
		if got != want {
			t.Fatalf("IsClaimed(%q) = %v, want %v", ch, got, want)
		}
	}
}
