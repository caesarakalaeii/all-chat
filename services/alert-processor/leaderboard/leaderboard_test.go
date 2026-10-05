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

package leaderboard

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) (*miniredis.Miniredis, *Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, New(rdb)
}

// TestCategory maps the amount-bearing event types onto the three leaderboard
// categories. Everything else (follows, shares, raids, subs without a gift
// count) must return false — a "follow" leaderboard would silently mix
// non-monetary noise into a gift ranking.
func TestCategory(t *testing.T) {
	cases := []struct {
		eventType  string
		wantCat    string
		wantExists bool
	}{
		{"bits", "bits", true},
		{"mystery_gift", "gifts", true},
		{"gift_subscription", "gifts", true},
		{"membership_gift", "gifts", true},
		{"gift", "gifts", true},
		{"super_chat", "super_chat", true},
		{"follow", "", false},
		{"share", "", false},
		{"raid", "", false},
		{"subscription", "", false},
		{"resubscription", "", false},
		{"channel_points", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		cat, ok := Category(tc.eventType)
		assert.Equal(t, tc.wantExists, ok, "Category(%q) existence", tc.eventType)
		assert.Equal(t, tc.wantCat, cat, "Category(%q) value", tc.eventType)
	}
}

// TestMember_PlatformScoped: the same numeric id on two platforms is two
// different people (Twitch "123" and YouTube "123" collide otherwise).
func TestMember_PlatformScoped(t *testing.T) {
	assert.Equal(t, "twitch:123", Member("twitch", "123"))
	assert.NotEqual(t, Member("twitch", "123"), Member("youtube", "123"))
}

// TestRecordAmount_AccumulatesPerOverlayAndCategory: the sorted set is the
// running SUM of amounts, keyed per overlay and category — a second gift must
// add to the first, not replace it, and must not leak into another overlay's
// board or another category.
func TestRecordAmount_AccumulatesPerOverlayAndCategory(t *testing.T) {
	mr, c := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, c.RecordAmount(ctx, "overlay-a", "gifts", "twitch:alice", 5))
	require.NoError(t, c.RecordAmount(ctx, "overlay-a", "gifts", "twitch:alice", 3))
	require.NoError(t, c.RecordAmount(ctx, "overlay-a", "gifts", "twitch:bob", 10))
	require.NoError(t, c.RecordAmount(ctx, "overlay-a", "bits", "twitch:alice", 100))
	require.NoError(t, c.RecordAmount(ctx, "overlay-b", "gifts", "twitch:alice", 1))

	alice, err := mr.ZScore("overlay:overlay-a:leaderboard:gifts", "twitch:alice")
	require.NoError(t, err)
	assert.InDelta(t, 8.0, alice, 1e-9, "gift amounts must sum into one score")

	bob, err := mr.ZScore("overlay:overlay-a:leaderboard:gifts", "twitch:bob")
	require.NoError(t, err)
	assert.InDelta(t, 10.0, bob, 1e-9)

	bits, err := mr.ZScore("overlay:overlay-a:leaderboard:bits", "twitch:alice")
	require.NoError(t, err, "bits must land on the bits board, not the gifts board")
	assert.InDelta(t, 100.0, bits, 1e-9)

	other, err := mr.ZScore("overlay:overlay-b:leaderboard:gifts", "twitch:alice")
	require.NoError(t, err, "boards must be scoped per overlay")
	assert.InDelta(t, 1.0, other, 1e-9)
}

// TestRecordAmount_RefreshesTTL: boards are a 30-day rolling window that stays
// alive while gifts keep coming — a write must reset the expiry, and a board
// nobody touches must eventually expire rather than live forever.
func TestRecordAmount_RefreshesTTL(t *testing.T) {
	mr, c := newTestClient(t)
	ctx := context.Background()
	key := "overlay:overlay-a:leaderboard:gifts"

	require.NoError(t, c.RecordAmount(ctx, "overlay-a", "gifts", "twitch:alice", 5))

	ttl := mr.TTL(key)
	assert.InDelta(t, (30 * 24 * time.Hour).Seconds(), ttl.Seconds(), 60, "fresh board TTL must be ~30 days")

	// Age the board almost to expiry, then write again: the write must extend
	// the life back to ~30 days.
	mr.FastForward(29 * 24 * time.Hour)
	require.NoError(t, c.RecordAmount(ctx, "overlay-a", "gifts", "twitch:alice", 1))

	ttl = mr.TTL(key)
	assert.InDelta(t, (30 * 24 * time.Hour).Seconds(), ttl.Seconds(), 60, "a write must refresh the 30-day TTL")

	// With no further writes the board expires entirely.
	mr.FastForward(31 * 24 * time.Hour)
	assert.False(t, mr.Exists(key), "an untouched board must expire after 30 days")
}
