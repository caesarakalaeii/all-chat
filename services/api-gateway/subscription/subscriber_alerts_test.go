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

package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type received struct{ overlayID, channel, payload string }

func newCapturingSubscriber(t *testing.T, mr *miniredis.Miniredis) (*Subscriber, *redis.Client, chan received) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	got := make(chan received, 8)
	sub := NewSubscriber(client, zap.NewNop(), func(overlayID, channel string, message []byte) {
		got <- received{overlayID, channel, string(message)}
	}, metrics.NewGatewayMetricsForTest())
	sub.lingerDuration = 0
	t.Cleanup(sub.Stop)
	return sub, client, got
}

func expectMessage(t *testing.T, got chan received, channel string) {
	t.Helper()
	select {
	case r := <-got:
		assert.Equal(t, channel, r.channel)
	case <-time.After(2 * time.Second):
		t.Fatalf("nothing received on %s", channel)
	}
}

func expectNothing(t *testing.T, got chan received) {
	t.Helper()
	select {
	case r := <-got:
		t.Fatalf("unexpected message on %s", r.channel)
	case <-time.After(200 * time.Millisecond):
	}
}

// An alerts/goal/list overlay additionally listens on overlay:{id}:alerts,
// the alert-processor's channel (ADR-0068); a chat overlay does not.
func TestSubscriberAlertsChannelIsConditional(t *testing.T) {
	mr := miniredis.RunT(t)
	sub, client, got := newCapturingSubscriber(t, mr)
	ctx := context.Background()

	require.NoError(t, sub.Subscribe(ctx, "alerts-overlay", true))
	require.NoError(t, sub.Subscribe(ctx, "chat-overlay", false))

	require.NoError(t, client.Publish(ctx, "overlay:alerts-overlay:alerts", "{}").Err())
	expectMessage(t, got, "overlay:alerts-overlay:alerts")

	require.NoError(t, client.Publish(ctx, "overlay:chat-overlay:alerts", "{}").Err())
	expectNothing(t, got)

	// The engagement siblings keep working on both.
	require.NoError(t, client.Publish(ctx, "overlay:alerts-overlay:poll", "{}").Err())
	expectMessage(t, got, "overlay:alerts-overlay:poll")
}

// A viewer-only socket may create the shared subscription first; a later
// alerts socket must still get the alerts channel added to it.
func TestSubscriberAddsAlertsChannelToAnExistingSubscription(t *testing.T) {
	mr := miniredis.RunT(t)
	sub, client, got := newCapturingSubscriber(t, mr)
	ctx := context.Background()

	require.NoError(t, sub.SubscribeViewerOnly(ctx, "overlay-1"))
	require.NoError(t, sub.Subscribe(ctx, "overlay-1", true))

	require.Eventually(t, func() bool {
		n, _ := client.PubSubNumSub(ctx, "overlay:overlay-1:alerts").Result()
		return n["overlay:overlay-1:alerts"] == 1
	}, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, client.Publish(ctx, "overlay:overlay-1:alerts", "{}").Err())
	expectMessage(t, got, "overlay:overlay-1:alerts")
}

func TestSubscriberChannelsFor(t *testing.T) {
	assert.Equal(t,
		[]string{"overlay:o", "overlay:o:updates", "overlay:o:poll", "overlay:o:prediction"},
		channelsFor("o", false, false))
	assert.Equal(t,
		[]string{"overlay:o", "overlay:o:updates", "overlay:o:poll", "overlay:o:prediction", "overlay:o:alerts"},
		channelsFor("o", false, true))
	assert.Equal(t,
		[]string{"overlay:o", "overlay:o:poll", "overlay:o:prediction"},
		channelsFor("o", true, false))
}
