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

package channels

import (
	"context"
	"sort"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func managerWithClaims(t *testing.T) (*miniredis.Miniredis, *twitchchat.ClaimStore, *Manager) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	store := twitchchat.NewClaimStore(rc)
	m := &Manager{logger: zap.NewNop()}
	m.SetChatClaimStore(store)
	return mr, store, m
}

// Channels EventSub is actively serving (have a live claim) must be excluded from the IRC desired
// set; everything else falls through to IRC (ADR-0015). Matching is case-insensitive.
func TestExcludeEventSubOwnedChannels_ExcludesClaimedOnly(t *testing.T) {
	_, store, m := managerWithClaims(t)
	ctx := context.Background()

	if err := store.Claim(ctx, "ownedchan", "1"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := store.Claim(ctx, "OwnedTwo", "2"); err != nil { // mixed case
		t.Fatalf("Claim: %v", err)
	}

	desired := []string{"OwnedChan", "freechan", "ownedtwo", "another"}
	got := m.excludeEventSubOwnedChannels(ctx, desired)

	sort.Strings(got)
	want := []string{"another", "freechan"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// With no live claims, IRC must read every desired channel.
func TestExcludeEventSubOwnedChannels_NoClaimsKeepsAll(t *testing.T) {
	_, _, m := managerWithClaims(t)
	desired := []string{"a", "b", "c"}
	got := m.excludeEventSubOwnedChannels(context.Background(), append([]string(nil), desired...))
	if len(got) != 3 {
		t.Fatalf("want all 3 channels kept, got %v", got)
	}
}

// Fail-open: a Redis error while reading claims must NOT exclude anything (IRC reads everything),
// so a Redis blip can never silently lose chat. message-processor dedupes the resulting overlap.
func TestExcludeEventSubOwnedChannels_FailsOpenOnRedisError(t *testing.T) {
	mr, store, m := managerWithClaims(t)
	ctx := context.Background()
	_ = store.Claim(ctx, "ownedchan", "1")
	mr.Close() // subsequent SCAN errors

	desired := []string{"ownedchan", "freechan"}
	got := m.excludeEventSubOwnedChannels(ctx, append([]string(nil), desired...))
	if len(got) != 2 {
		t.Fatalf("fail-open must keep all channels on Redis error; got %v", got)
	}
}

// A nil claim store (e.g. no Redis in tests) disables the filter — IRC reads everything.
func TestExcludeEventSubOwnedChannels_NilStoreKeepsAll(t *testing.T) {
	m := &Manager{logger: zap.NewNop()} // chatClaims left nil
	desired := []string{"a", "b"}
	got := m.excludeEventSubOwnedChannels(context.Background(), desired)
	if len(got) != 2 {
		t.Fatalf("nil store must keep all channels; got %v", got)
	}
}
