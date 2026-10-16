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

package goals

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeLister struct{ goals []repository.PolledGoal }

func (f *fakeLister) PolledGoals(context.Context) ([]repository.PolledGoal, error) {
	return f.goals, nil
}

type fakeCounter struct {
	mu    sync.Mutex
	n     int64
	err   error
	calls int
}

func (f *fakeCounter) Count(context.Context, repository.PolledGoal) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.n, f.err
}

var followerGoal = repository.PolledGoal{
	OverlayID: "ov", OwnerUserID: "user-1", Metric: "followers", Platform: "twitch",
	ChannelID: "streamer", Target: 500, Title: "Followers",
}

func newPoller(t *testing.T, counter Counter) (*miniredis.Miniredis, *redis.Client, *Poller) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	p := NewPoller(&fakeLister{goals: []repository.PolledGoal{followerGoal}},
		map[string]Counter{"twitch": counter}, rdb, publisher.New(rdb), time.Minute, zap.NewNop())
	return mr, rdb, p
}

// No overlay:connected key (the gateway's demand signal): no API call at all.
func TestPollOnce_SkipsOverlaysThatAreNotConnected(t *testing.T) {
	counter := &fakeCounter{n: 42}
	mr, _, p := newPoller(t, counter)

	p.PollOnce(context.Background())
	assert.Zero(t, counter.calls, "a disconnected overlay must not spend an API call")
	assert.False(t, mr.Exists(alerts.GoalKey("ov", "followers")))
}

func TestPollOnce_StoresAndPushesTheCount(t *testing.T) {
	counter := &fakeCounter{n: 321}
	mr, rdb, p := newPoller(t, counter)
	require.NoError(t, mr.Set("overlay:connected:ov", "1"))
	require.NoError(t, mr.Set(alerts.GoalStatusKey("ov"), "reconnect"))
	sub := rdb.Subscribe(context.Background(), publisher.Channel("ov"))
	t.Cleanup(func() { _ = sub.Close() })
	_, err := sub.Receive(context.Background())
	require.NoError(t, err)

	p.PollOnce(context.Background())

	v, err := mr.Get(alerts.GoalKey("ov", "followers"))
	require.NoError(t, err)
	assert.Equal(t, "321", v)
	assert.False(t, mr.Exists(alerts.GoalStatusKey("ov")), "a good poll clears a stale reconnect status")
	select {
	case m := <-sub.Channel():
		assert.JSONEq(t, `{"type":"goal","overlay_id":"ov","goal":"followers","current":321,"target":500,"title":"Followers"}`, m.Payload)
	case <-time.After(time.Second):
		t.Fatal("no goal frame")
	}
}

// Two replicas tick on their own schedules: the per-overlay lock lets only
// one of them poll a goal per interval.
func TestPollOnce_OneReplicaPollsPerInterval(t *testing.T) {
	counter := &fakeCounter{n: 1}
	mr, rdb, a := newPoller(t, counter)
	b := NewPoller(&fakeLister{goals: []repository.PolledGoal{followerGoal}},
		map[string]Counter{"twitch": counter}, rdb, publisher.New(rdb), time.Minute, zap.NewNop())
	require.NoError(t, mr.Set("overlay:connected:ov", "1"))

	a.PollOnce(context.Background())
	b.PollOnce(context.Background())
	assert.Equal(t, 1, counter.calls, "the second replica must not double-poll")

	mr.FastForward(time.Minute)
	b.PollOnce(context.Background())
	assert.Equal(t, 2, counter.calls, "the next interval polls again")
}

func TestPollOnce_RecordsDegradedStates(t *testing.T) {
	for err, want := range map[error]string{ErrReconnect: "reconnect", ErrHidden: "hidden"} {
		counter := &fakeCounter{err: err}
		mr, _, p := newPoller(t, counter)
		require.NoError(t, mr.Set("overlay:connected:ov", "1"))
		require.NoError(t, mr.Set(alerts.GoalKey("ov", "followers"), "10"))

		p.PollOnce(context.Background())
		status, gerr := mr.Get(alerts.GoalStatusKey("ov"))
		require.NoError(t, gerr)
		assert.Equal(t, want, status)
		v, _ := mr.Get(alerts.GoalKey("ov", "followers"))
		assert.Equal(t, "10", v, "a failed poll keeps the last good count")
	}

	counter := &fakeCounter{err: errors.New("helix 500")}
	mr, _, p := newPoller(t, counter)
	require.NoError(t, mr.Set("overlay:connected:ov", "1"))
	p.PollOnce(context.Background())
	assert.False(t, mr.Exists(alerts.GoalStatusKey("ov")), "a transient error is not a user-facing state")
}
