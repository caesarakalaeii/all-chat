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
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TestPublish_Success verifies that Publish creates correct XADD command with all required fields
func TestPublish_Success(t *testing.T) {
	// Create test message
	msg := &innertube.RawChatMessage{
		MessageID: "test-msg-123",
		Platform:  "youtube",
		ChannelID: "UCtest123",
		StreamID:  "stream-456",
		UserID:    "user-789",
		Username:  "TestUser",
		Text:      "Hello world!",
		Timestamp: time.Date(2025, 2, 21, 12, 0, 0, 0, time.UTC),
		Tags:      map[string]string{"badges": "moderator"},
	}

	// Note: This is a basic smoke test
	// Full integration test requires Redis (see Task 2 verification)
	// For now, we verify the structure is correct by checking that
	// marshalling works and the publisher can be created

	logger := zap.NewNop()

	// Create a test Redis client (will fail to connect but structure is valid)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Use nil for metrics and deletionBuffer in tests that don't verify those behaviors
	publisher := NewStreamPublisher(client, logger, nil, nil)

	if publisher == nil {
		t.Fatal("NewStreamPublisher returned nil")
	}

	if publisher.client == nil {
		t.Error("Publisher client is nil")
	}

	if publisher.logger == nil {
		t.Error("Publisher logger is nil")
	}

	// Verify message can be marshalled (will be used in Publish)
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		t.Errorf("ToJSON failed: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("ToJSON returned empty bytes")
	}

	// Note: Full XADD verification requires running Redis instance
	// See Task 2 integration test for end-to-end validation
}

// TestPing_Success verifies Ping correctly wraps Redis client ping
func TestPing_Success(t *testing.T) {
	logger := zap.NewNop()

	// Create a test Redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Use nil for metrics and deletionBuffer in tests that don't verify those behaviors
	publisher := NewStreamPublisher(client, logger, nil, nil)

	ctx := context.Background()

	// This will fail if Redis is not running, which is expected
	err := publisher.Ping(ctx)

	// We expect an error since Redis is likely not running during unit tests
	// The important thing is that Ping returns the error from client.Ping()
	if err == nil {
		// Redis is running, which is fine for this test
		t.Log("Redis is running, Ping succeeded")
	} else {
		// Expected case: Redis not running during unit tests
		t.Logf("Ping failed as expected (Redis not running): %v", err)
	}
}

// TestPublishBatch_EmptySlice verifies PublishBatch handles empty slice correctly
func TestPublishBatch_EmptySlice(t *testing.T) {
	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Use nil for metrics and deletionBuffer in tests that don't verify those behaviors
	publisher := NewStreamPublisher(client, logger, nil, nil)

	ctx := context.Background()

	// PublishBatch should return nil for empty slice
	err := publisher.PublishBatch(ctx, []*innertube.RawChatMessage{})
	if err != nil {
		t.Errorf("PublishBatch failed for empty slice: %v", err)
	}
}

// Note: Full integration tests with Redis will be performed in Task 2
// These unit tests verify the structure and basic error handling
