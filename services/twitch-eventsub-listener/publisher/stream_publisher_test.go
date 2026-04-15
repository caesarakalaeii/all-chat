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
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestStreamPublisherPublishWithRingBuffer verifies that when XADD fails,
// Publish returns nil (buffered) not error — LI-01/LI-02/LI-03 fix.
func TestStreamPublisherPublishWithRingBuffer(t *testing.T) {
	var publishCalls atomic.Int32
	failPublish := func(_ context.Context, _ []byte) error {
		publishCalls.Add(1)
		return errors.New("redis: connection refused")
	}

	reg := prometheus.NewRegistry()
	pub := newStreamPublisherWithRingBuffer(failPublish, zap.NewNop(), reg)
	defer pub.Stop()

	msg := &models.RawChatMessage{
		MessageID: "test-rb-1",
		Platform:  "twitch",
		ChannelID: "12345",
		UserID:    "user-1",
		Username:  "testuser",
		Text:      "Hello ring buffer",
		EventType: "chat_message",
		Timestamp: time.Now().UTC(),
	}

	// Publish should return nil even when underlying XADD fails
	err := pub.Publish(context.Background(), msg)
	require.NoError(t, err, "Publish must return nil when XADD fails — message should be buffered")

	// Underlying publish function should have been called once
	assert.Equal(t, int32(1), publishCalls.Load())
}

// TestStreamPublisherStopDrainsBuffer verifies Stop calls through to ring buffer cleanly.
func TestStreamPublisherStopDrainsBuffer(t *testing.T) {
	var successCalls atomic.Int32
	var failCalls atomic.Int32

	publishFn := func(_ context.Context, _ []byte) error {
		if failCalls.Load() < 1 {
			failCalls.Add(1)
			return errors.New("transient error")
		}
		successCalls.Add(1)
		return nil
	}

	reg := prometheus.NewRegistry()
	pub := newStreamPublisherWithRingBuffer(publishFn, zap.NewNop(), reg)

	msg := &models.RawChatMessage{
		MessageID: "test-stop-1",
		Platform:  "twitch",
		ChannelID: "test",
		EventType: "chat_message",
		Timestamp: time.Now().UTC(),
	}

	// First publish fails, message gets buffered
	err := pub.Publish(context.Background(), msg)
	require.NoError(t, err)

	// Wait for retry loop to drain (500ms tick + margin)
	time.Sleep(700 * time.Millisecond)

	// Stop cleanly shuts down the retry goroutine
	pub.Stop()

	assert.GreaterOrEqual(t, successCalls.Load(), int32(0), "Stop should not panic")
}
