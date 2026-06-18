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

package testgen

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/publisher"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// testProcessorMetrics is registered once for the whole package: a second
// NewProcessorMetrics() would panic on duplicate Prometheus registration.
var testProcessorMetrics = metrics.NewProcessorMetrics()

// TestEndToEndPublishesToRedis verifies the full publish path: a real
// PubSubPublisher writes generator output to overlay:{id} and a subscriber
// receives correctly-shaped UnifiedChatMessage payloads. Set REDIS_ADDR to run
// (e.g. REDIS_ADDR=localhost:6379 go test ./testgen -run EndToEnd).
func TestEndToEndPublishesToRedis(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set; skipping Redis end-to-end test")
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	channel := "overlay:" + testOverlayID
	sub := rdb.Subscribe(ctx, channel)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	ch := sub.Channel()

	pub := publisher.NewPubSubPublisher(rdb, zap.NewNop(), testProcessorMetrics)
	g := NewGenerator(testOverlayID, pub, nil, nil, zap.NewNop())

	if _, err := g.Start(Config{DurationSeconds: 3, RatePerSecond: 20, VoteRatio: 0.5, EventEveryN: 4}); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer g.Stop()

	var got, votes, events int
	deadline := time.After(4 * time.Second)
loop:
	for {
		select {
		case m := <-ch:
			var msg models.UnifiedChatMessage
			if err := json.Unmarshal([]byte(m.Payload), &msg); err != nil {
				t.Fatalf("payload not valid UnifiedChatMessage json: %v", err)
			}
			if msg.OverlayID != testOverlayID {
				t.Fatalf("overlay id = %q", msg.OverlayID)
			}
			got++
			switch msg.Message.Text {
			case "1", "2", "3", "4":
				votes++
			}
			if msg.Event != nil {
				events++
			}
			if got >= 20 {
				break loop
			}
		case <-deadline:
			break loop
		}
	}

	if got == 0 {
		t.Fatal("received no messages on the overlay channel")
	}
	if votes == 0 {
		t.Fatal("expected at least one poll-vote message")
	}
	if events == 0 {
		t.Fatal("expected at least one event message")
	}
	t.Logf("received %d messages (%d votes, %d events)", got, votes, events)
}
