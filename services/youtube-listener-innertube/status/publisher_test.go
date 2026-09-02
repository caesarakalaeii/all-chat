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

package status

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// newTestPublisher wires a Publisher to an in-process Redis and returns both, so a
// test can assert on what the publish actually left behind. logs captures everything
// the publisher logged, for the failure paths that are only observable there.
func newTestPublisher(t *testing.T) (*Publisher, *miniredis.Miniredis, *redis.Client, *observer.ObservedLogs) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	core, logs := observer.New(zap.WarnLevel)
	return NewPublisher(client, zap.New(core)), server, client, logs
}

// A parked channel's status arrives an hour after the streamer went offline, when
// nobody is subscribed, so Pub/Sub alone drops it. The snapshot is what a dashboard
// opened the next day can still read.
func TestPublishWritesSnapshotWithTTL(t *testing.T) {
	publisher, server, client, _ := newTestPublisher(t)
	ctx := context.Background()

	publisher.Publish(ctx, Message{
		Platform:     "youtube",
		ChannelID:    "UCabc123",
		Status:       "paused",
		ErrorMessage: "No live stream found after 1h",
	})

	const key = "platform:status:youtube:UCabc123"
	snapshot, err := client.Get(ctx, key).Result()
	require.NoError(t, err, "snapshot key %s must exist after a publish", key)
	assert.JSONEq(t, `{
		"platform": "youtube",
		"channel_id": "UCabc123",
		"status": "paused",
		"error_message": "No live stream found after 1h"
	}`, snapshot)

	assert.Equal(t, SnapshotTTL, server.TTL(key))
}

// The PUBLISH is the live path and must be untouched by the snapshot work: a monitor
// that is already open keeps updating in real time.
func TestPublishStillDeliversToSubscribers(t *testing.T) {
	publisher, _, client, _ := newTestPublisher(t)
	ctx := context.Background()

	subscription := client.Subscribe(ctx, PlatformStatusChannel)
	t.Cleanup(func() { _ = subscription.Close() })
	_, err := subscription.Receive(ctx)
	require.NoError(t, err)

	publisher.Publish(ctx, Message{Platform: "youtube", ChannelID: "UCabc123", Status: "paused"})

	select {
	case msg := <-subscription.Channel():
		assert.JSONEq(t, `{"platform":"youtube","channel_id":"UCabc123","status":"paused"}`, msg.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("no message delivered on " + PlatformStatusChannel)
	}
}

// Redis runs HA with min-replicas-to-write 1, so a write can pause or fail under node
// loss. Status persistence is best-effort: a failed snapshot must warn and nothing else,
// so the live PUBLISH still reaches an open monitor.
func TestPublishWarnsButStillDeliversWhenSnapshotWriteFails(t *testing.T) {
	publisher, _, client, logs := newTestPublisher(t)
	ctx := context.Background()

	subscription := client.Subscribe(ctx, PlatformStatusChannel)
	t.Cleanup(func() { _ = subscription.Close() })
	_, err := subscription.Receive(ctx)
	require.NoError(t, err)

	// A list at the snapshot key makes SET fail with WRONGTYPE — a rejected snapshot
	// write, without breaking the connection the PUBLISH also uses.
	require.NoError(t, client.RPush(ctx, "platform:status:youtube:UCabc123", "not-a-string").Err())

	publisher.Publish(ctx, Message{Platform: "youtube", ChannelID: "UCabc123", Status: "paused"})

	select {
	case msg := <-subscription.Channel():
		assert.JSONEq(t, `{"platform":"youtube","channel_id":"UCabc123","status":"paused"}`, msg.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("snapshot failure suppressed the publish on " + PlatformStatusChannel)
	}

	require.Len(t, logs.All(), 1)
	entry := logs.All()[0]
	assert.Equal(t, zap.WarnLevel, entry.Level)
	assert.Contains(t, entry.Message, "snapshot")
}
