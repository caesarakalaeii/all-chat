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

package consumer

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newCountTest(t *testing.T) (*ChatCommandConsumer, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	store := &fakeCmdStore{owners: []repository.ChannelCommands{
		{UserID: ownerID, Owned: true, TimerMessages: true},
	}, uses: map[uuid.UUID]int64{}}
	return NewChatCommandConsumer(rdb, store, &fakeOutput{}, "test", zap.NewNop()), rdb
}

// One unique chat line on the owner's own channel counts once toward their timer.
func TestCountTimerLine_CountsUniqueLines(t *testing.T) {
	c, rdb := newCountTest(t)
	ctx := context.Background()

	c.handle(ctx, job("m1", "twitch", "hello"))
	c.handle(ctx, job("m2", "twitch", "next line"))

	n, err := rdb.Get(ctx, chattimerLinesKey(ownerID)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

// Kick publishes one raw message per overlay carrying the channel, all stamped with
// the same message id: a second copy of the same line must not count again.
func TestCountTimerLine_KickCopiesCountOnce(t *testing.T) {
	c, rdb := newCountTest(t)
	ctx := context.Background()

	c.handle(ctx, job("kick-1", "kick", "hello there"))
	c.handle(ctx, job("kick-1", "kick", "hello there"))

	n, err := rdb.Get(ctx, chattimerLinesKey(ownerID)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

// A line whose text matches an echo the streamer's channel just posted (a reply or
// a timer post) is their own output, not a viewer's line, and must not count.
func TestCountTimerLine_EchoDoesNotCount(t *testing.T) {
	c, rdb := newCountTest(t)
	ctx := context.Background()

	c.RegisterEcho(ctx, ownerID, "hello {user}")
	c.handle(ctx, job("m1", "twitch", "hello {user}"))
	c.handle(ctx, job("m2", "twitch", "real line"))

	n, err := rdb.Get(ctx, chattimerLinesKey(ownerID)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "only the non-echo line counts")
}

// The repository's counts flag already encodes "this channel is the owner's OWN,
// and they run an enabled message-counting timer". An owner watching a foreign
// channel (Owned=false, TimerMessages=false) must not accumulate a counter there.
func TestCountTimerLine_ForeignChannelDoesNotCount(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	foreign := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	store := &fakeCmdStore{owners: []repository.ChannelCommands{
		{UserID: foreign, Owned: false, TimerMessages: false},
		{UserID: ownerID, Owned: true, TimerMessages: true},
	}, uses: map[uuid.UUID]int64{}}
	c := NewChatCommandConsumer(rdb, store, &fakeOutput{}, "test", zap.NewNop())

	c.handle(context.Background(), job("m1", "twitch", "hello"))

	own, err := rdb.Get(context.Background(), chattimerLinesKey(ownerID)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(1), own)
	exists, err := rdb.Exists(context.Background(), chattimerLinesKey(foreign)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "no counter for the owner who only carries the channel")
}
