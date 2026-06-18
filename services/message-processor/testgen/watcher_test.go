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
	"os"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/publisher"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TestDemandWatcherFollowsPresence verifies the generator auto-starts when the
// api-gateway presence key appears and stops when it disappears. Set REDIS_ADDR
// to run (e.g. REDIS_ADDR=localhost:6379 go test ./testgen -run DemandWatcher).
func TestDemandWatcherFollowsPresence(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set; skipping Redis demand-watcher test")
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	key := presenceKeyPrefix + testOverlayID
	rdb.Del(ctx, key)
	defer rdb.Del(ctx, key)

	pub := publisher.NewPubSubPublisher(rdb, zap.NewNop(), testProcessorMetrics)
	gen := NewGenerator(testOverlayID, pub, nil, nil, zap.NewNop())
	watcher := NewDemandWatcher(testOverlayID, rdb, gen, Config{RatePerSecond: 20}, zap.NewNop())

	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go watcher.Run(wctx)

	// No presence yet -> generator idle.
	time.Sleep(300 * time.Millisecond)
	if running, _ := gen.State(); running {
		t.Fatal("generator should be idle before any connection")
	}

	// Simulate a client connecting: set presence key + publish the connect event
	// so the watcher reconciles immediately instead of waiting for the ticker.
	rdb.Set(ctx, key, "1", time.Minute)
	rdb.Publish(ctx, connectionsChannel, `{"type":"connected","overlay_id":"`+testOverlayID+`"}`)

	if !waitFor(2*time.Second, func() bool { r, m := gen.State(); return r && m == modeDemand }) {
		t.Fatal("generator did not start on connection presence")
	}

	// Simulate disconnect: drop the key + publish the event.
	rdb.Del(ctx, key)
	rdb.Publish(ctx, connectionsChannel, `{"type":"disconnected","overlay_id":"`+testOverlayID+`"}`)

	if !waitFor(2*time.Second, func() bool { r, _ := gen.State(); return !r }) {
		t.Fatal("generator did not stop after disconnect")
	}
}

func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
