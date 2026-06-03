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

package dedup

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newNativeTestDedup(t *testing.T) (*miniredis.Miniredis, *Deduplicator) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return mr, NewDeduplicator(rc, zap.NewNop())
}

func TestIsDuplicateNativeID_FirstSeenThenDuplicate(t *testing.T) {
	_, d := newNativeTestDedup(t)
	ctx := context.Background()

	dup, err := d.IsDuplicateNativeID(ctx, "twitch", "abc")
	if err != nil || dup {
		t.Fatalf("first sighting: dup=%v err=%v, want false/nil", dup, err)
	}
	dup, err = d.IsDuplicateNativeID(ctx, "twitch", "abc")
	if err != nil || !dup {
		t.Fatalf("second sighting: dup=%v err=%v, want true/nil", dup, err)
	}
}

func TestIsDuplicateNativeID_NamespacedByPlatform(t *testing.T) {
	_, d := newNativeTestDedup(t)
	ctx := context.Background()
	_, _ = d.IsDuplicateNativeID(ctx, "twitch", "shared-id")
	// Same id under a different platform must be independent (key is platform-scoped).
	if dup, _ := d.IsDuplicateNativeID(ctx, "kick", "shared-id"); dup {
		t.Fatal("native id must be namespaced by platform")
	}
}

func TestIsDuplicateNativeID_EmptyIDNeverDuplicate(t *testing.T) {
	_, d := newNativeTestDedup(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if dup, err := d.IsDuplicateNativeID(ctx, "twitch", ""); dup || err != nil {
			t.Fatalf("empty id must never be a duplicate; got dup=%v err=%v", dup, err)
		}
	}
}

func TestIsDuplicateNativeID_FailsOpenOnRedisError(t *testing.T) {
	mr, d := newNativeTestDedup(t)
	mr.Close() // force a connection error
	dup, err := d.IsDuplicateNativeID(context.Background(), "twitch", "abc")
	if dup {
		t.Fatal("on Redis error the message must be treated as NOT a duplicate (fail open)")
	}
	if err == nil {
		t.Fatal("expected the underlying Redis error to be returned for observability")
	}
}
