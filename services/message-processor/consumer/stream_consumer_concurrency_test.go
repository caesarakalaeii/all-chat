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
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestNewStreamConsumer_DefaultConcurrency verifies the constructor applies the
// bounded-concurrency default so the pipeline is never accidentally serial.
func TestNewStreamConsumer_DefaultConcurrency(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c := newFullTestConsumer(t, mr, func(context.Context, *models.RawChatMessage) error { return nil })
	assert.Equal(t, DefaultProcessConcurrency, c.concurrency)
}

// TestStreamConsumer_SetProcessConcurrency verifies the setter clamps sub-1 values
// to strictly-sequential (1) rather than 0 (which would deadlock the worker pool).
func TestStreamConsumer_SetProcessConcurrency(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c := newFullTestConsumer(t, mr, func(context.Context, *models.RawChatMessage) error { return nil })

	c.SetProcessConcurrency(32)
	assert.Equal(t, 32, c.concurrency)

	c.SetProcessConcurrency(0)
	assert.Equal(t, 1, c.concurrency, "sub-1 concurrency must clamp to 1")

	c.SetProcessConcurrency(-5)
	assert.Equal(t, 1, c.concurrency, "negative concurrency must clamp to 1")
}

// TestStreamConsumer_ProcessesBatchConcurrently proves a read batch is processed in
// parallel (bounded by concurrency) rather than strictly one-at-a-time, so a slow
// enrichment I/O on one message cannot stall the rest of the batch. This is the core
// throughput fix: it fails against the old sequential loop (max in-flight == 1).
func TestStreamConsumer_ProcessesBatchConcurrently(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	ctx := context.Background()

	const total = 12
	const concurrency = 4
	for i := 0; i < total; i++ {
		_, err = client.XAdd(ctx, &redis.XAddArgs{
			Stream: StreamKey,
			ID:     "*",
			Values: map[string]interface{}{"data": `{"message_id":"m","platform":"twitch","channel_id":"ch1","timestamp":"2026-01-01T00:00:00Z"}`},
		}).Result()
		require.NoError(t, err)
	}
	require.NoError(t, client.XGroupCreateMkStream(ctx, StreamKey, ConsumerGroup, "0").Err())

	var inFlight, maxInFlight, handled int32
	c := &StreamConsumer{
		client:  client,
		logger:  zaptest.NewLogger(t),
		metrics: sharedTestMetrics,
		handler: func(context.Context, *models.RawChatMessage) error {
			cur := atomic.AddInt32(&inFlight, 1)
			for { // lock-free running maximum
				old := atomic.LoadInt32(&maxInFlight)
				if cur <= old || atomic.CompareAndSwapInt32(&maxInFlight, old, cur) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond) // simulate slow enrichment I/O
			atomic.AddInt32(&handled, 1)
			atomic.AddInt32(&inFlight, -1)
			return nil
		},
		consumerName:   "test-consumer",
		stopCh:         make(chan struct{}),
		msgIDRegistry:  registry.NewRedisRegistry(client, time.Hour),
		deletionBuffer: registry.NewRedisDeletionBuffer(client, time.Hour),
		concurrency:    concurrency,
	}

	require.NoError(t, c.readAndProcess(ctx))

	assert.Equal(t, int32(total), atomic.LoadInt32(&handled), "every message must be processed")
	assert.Greater(t, atomic.LoadInt32(&maxInFlight), int32(1), "batch must be processed in parallel, not serially")
	assert.LessOrEqual(t, atomic.LoadInt32(&maxInFlight), int32(concurrency), "parallelism must stay bounded")

	pending, err := client.XPending(ctx, StreamKey, ConsumerGroup).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), pending.Count, "all messages must be ACKed regardless of concurrency")
}
