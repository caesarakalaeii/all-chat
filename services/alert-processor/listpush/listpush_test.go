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

package listpush

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setup(t *testing.T) (*miniredis.Miniredis, *redis.Client, *redis.PubSub) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	sub := rdb.Subscribe(context.Background(), publisher.Channel("ov"))
	t.Cleanup(func() { _ = sub.Close() })
	_, err := sub.Receive(context.Background())
	require.NoError(t, err)
	return mr, rdb, sub
}

func receive(t *testing.T, sub *redis.PubSub) *alerts.ListUpdateFrame {
	t.Helper()
	select {
	case msg := <-sub.Channel():
		var f alerts.ListUpdateFrame
		require.NoError(t, json.Unmarshal([]byte(msg.Payload), &f))
		return &f
	case <-time.After(300 * time.Millisecond):
		return nil
	}
}

// Many score changes inside one window collapse into one list_update carrying
// the board as it stands at flush time.
func TestFlush_CoalescesChangesIntoOneUpdate(t *testing.T) {
	_, rdb, sub := setup(t)
	ctx := context.Background()
	d := New(rdb, publisher.New(rdb), 2*time.Second, zap.NewNop())

	rdb.ZAdd(ctx, alerts.LeaderboardKey("ov", "bits"), redis.Z{Score: 100, Member: "twitch:1"})
	d.MarkDirty("ov", "bits")
	d.MarkDirty("ov", "bits")
	rdb.ZAdd(ctx, alerts.LeaderboardKey("ov", "bits"), redis.Z{Score: 300, Member: "twitch:2"})
	d.MarkDirty("ov", "bits")

	d.Flush(ctx)
	f := receive(t, sub)
	require.NotNil(t, f)
	assert.Equal(t, "list_update", f.Type)
	assert.Equal(t, "bits", f.Category)
	require.Len(t, f.Entries, 2)
	assert.Equal(t, "2", f.Entries[0].Name)
	assert.Nil(t, receive(t, sub), "one update per window, not one per change")

	d.Flush(ctx)
	assert.Nil(t, receive(t, sub), "nothing changed since: nothing to send")
}

// A second replica holding the window for this board defers the update
// instead of dropping it: the change goes out once the window has passed.
func TestFlush_DefersWhileAnotherReplicaHoldsTheWindow(t *testing.T) {
	mr, rdb, sub := setup(t)
	ctx := context.Background()
	a := New(rdb, publisher.New(rdb), 2*time.Second, zap.NewNop())
	b := New(rdb, publisher.New(rdb), 2*time.Second, zap.NewNop())

	a.MarkDirty("ov", "gifts")
	a.Flush(ctx)
	require.NotNil(t, receive(t, sub))

	b.MarkDirty("ov", "gifts")
	b.Flush(ctx)
	assert.Nil(t, receive(t, sub), "inside a's window: deferred")

	mr.FastForward(2 * time.Second)
	b.Flush(ctx)
	assert.NotNil(t, receive(t, sub), "the deferred change is sent, not lost")
}
