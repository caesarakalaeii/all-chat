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

package claimexport

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/shared/twitchchat"
)

const testService = "twitch-eventsub-listener"

func newTestStore(t *testing.T) (*twitchchat.ClaimStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rc.Close()
		mr.Close()
	})
	return twitchchat.NewClaimStore(rc), mr
}

// A freshly-migrated (EventSub-owned) channel is reported by the gauge, so the
// dashboard can subtract it from "still on IRC".
func TestSyncOnce_MirrorsClaimedChannels(t *testing.T) {
	chatOwned.Reset()
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Claim(ctx, "tangerinemonkey", "764844090"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := store.Claim(ctx, "mystixx", "123"); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	prev := syncOnce(ctx, store, testService, map[string]struct{}{}, zap.NewNop())

	if got := testutil.ToFloat64(chatOwned.WithLabelValues(testService, "tangerinemonkey")); got != 1 {
		t.Fatalf("tangerinemonkey gauge = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(chatOwned); n != 2 {
		t.Fatalf("series count = %d, want 2", n)
	}
	if _, ok := prev["tangerinemonkey"]; !ok {
		t.Fatalf("returned set should contain tangerinemonkey, got %v", prev)
	}
}

// When a claim lapses/releases, its series is dropped so the gauge never reports
// stale ownership (which would wrongly keep a re-IRC'd channel hidden).
func TestSyncOnce_DropsReleasedClaims(t *testing.T) {
	chatOwned.Reset()
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Claim(ctx, "tangerinemonkey", "1"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := store.Claim(ctx, "mystixx", "2"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	prev := syncOnce(ctx, store, testService, map[string]struct{}{}, zap.NewNop())

	if err := store.Release(ctx, "tangerinemonkey"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	syncOnce(ctx, store, testService, prev, zap.NewNop())

	if n := testutil.CollectAndCount(chatOwned); n != 1 {
		t.Fatalf("after release, series count = %d, want 1 (only mystixx)", n)
	}
	if got := testutil.ToFloat64(chatOwned.WithLabelValues(testService, "mystixx")); got != 1 {
		t.Fatalf("mystixx gauge = %v, want 1", got)
	}
}

// A nil store is a no-op (e.g. Redis-less test/deploy) and must not panic.
func TestExportOwnedChannels_NilStoreNoop(t *testing.T) {
	ExportOwnedChannels(context.Background(), nil, testService, zap.NewNop())
}
