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

package publisher

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/alert-processor/models"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestChannel_Naming pins the Pub/Sub channel contract with the API gateway /
// alerts overlay: overlay:{overlay_id}:alerts.
func TestChannel_Naming(t *testing.T) {
	assert.Equal(t, "overlay:overlay-a:alerts", Channel("overlay-a"))
}

// TestPublish_DeliversAlertEnvelopeToOverlayChannel publishes while a
// subscriber listens and asserts the delivered bytes are the alert envelope —
// this is the wire the alerts-overlay frontend parses.
func TestPublish_DeliversAlertEnvelopeToOverlayChannel(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	sub := rdb.Subscribe(context.Background(), Channel("overlay-a"))
	t.Cleanup(func() { _ = sub.Close() })
	// Flush the subscription confirmation so the first Channel() read is data.
	_, err = sub.Receive(context.Background())
	require.NoError(t, err)

	alert := &models.Alert{
		AlertID:    "alert-uuid",
		OverlayID:  "overlay-a",
		Platform:   "twitch",
		EventType:  "bits",
		EventData:  &mpmodels.EventInfo{Type: "bits"},
		User:       models.AlertUser{ID: "user-1", Name: "SomeChatter"},
		OccurredAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}

	p := New(rdb, zap.NewNop())
	require.NoError(t, p.Publish(context.Background(), alert))

	select {
	case msg := <-sub.Channel():
		assert.Equal(t, Channel("overlay-a"), msg.Channel)
		var got models.Alert
		require.NoError(t, json.Unmarshal([]byte(msg.Payload), &got))
		assert.Equal(t, *alert, got, "published payload must be the alert envelope")
	case <-time.After(2 * time.Second):
		t.Fatal("no alert arrived on overlay:{id}:alerts")
	}
}

// TestPublish_PropagatesRedisError: a publish failure must surface as an error
// so the consumer leaves the stream entry unacked and it is redelivered —
// swallowing it would drop the alert.
func TestPublish_PropagatesRedisError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	p := New(rdb, zap.NewNop())
	mr.Close() // simulate Redis going away mid-pipeline

	err = p.Publish(context.Background(), &models.Alert{OverlayID: "overlay-a"})
	assert.Error(t, err, "a failed publish must propagate to the caller")
}