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

package poller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestDetectOffline_EmptyContinuation verifies offline detection with empty continuation array
func TestDetectOffline_EmptyContinuation(t *testing.T) {
	resp := &innertube.LiveChatResponse{
		ContinuationContents: innertube.ContinuationContents{
			LiveChatContinuation: innertube.LiveChatContinuation{
				Continuations: []innertube.Continuation{}, // Empty = offline
			},
		},
	}

	isOffline := DetectOffline(resp)
	assert.True(t, isOffline, "Empty continuation array should indicate offline")
}

// TestDetectOffline_NilContinuation verifies offline detection with nil continuation array
func TestDetectOffline_NilContinuation(t *testing.T) {
	resp := &innertube.LiveChatResponse{
		ContinuationContents: innertube.ContinuationContents{
			LiveChatContinuation: innertube.LiveChatContinuation{
				Continuations: nil, // Nil = offline
			},
		},
	}

	isOffline := DetectOffline(resp)
	assert.True(t, isOffline, "Nil continuation array should indicate offline")
}

// TestDetectOffline_NilResponse verifies offline detection with nil response
func TestDetectOffline_NilResponse(t *testing.T) {
	isOffline := DetectOffline(nil)
	assert.True(t, isOffline, "Nil response should indicate offline")
}

// TestDetectOffline_ValidContinuation verifies online detection with valid continuation
func TestDetectOffline_ValidContinuation(t *testing.T) {
	resp := &innertube.LiveChatResponse{
		ContinuationContents: innertube.ContinuationContents{
			LiveChatContinuation: innertube.LiveChatContinuation{
				Continuations: []innertube.Continuation{
					{
						TimedContinuationData: &innertube.TimedContinuationData{
							Continuation: "valid_continuation_token",
						},
					},
				},
			},
		},
	}

	isOffline := DetectOffline(resp)
	assert.False(t, isOffline, "Valid continuation array should indicate online")
}

// TestHandleStreamOffline_DeletesMapping verifies Redis mapping deletion on offline
func TestHandleStreamOffline_DeletesMapping(t *testing.T) {
	// Setup Redis test client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test DB
	})
	defer redisClient.Close()

	// Check Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for integration test")
	}

	logger := zap.NewNop()
	repo := NewRepository(redisClient, logger)

	// Setup: Create a channel mapping
	channelID := "test_channel_offline"
	videoID := "test_video_123"
	err := repo.SetChannelVideoMapping(ctx, channelID, videoID)
	require.NoError(t, err)

	// Verify mapping exists
	cachedVideoID, err := repo.GetChannelVideoMapping(ctx, channelID)
	require.NoError(t, err)
	assert.Equal(t, videoID, cachedVideoID)

	// Execute: Handle stream offline (nil publisher — no lifecycle publish in integration test)
	err = HandleStreamOffline(ctx, channelID, videoID, repo, nil, logger)
	assert.Error(t, err, "HandleStreamOffline should return error to signal polling stop")
	assert.Contains(t, err.Error(), "stream offline")

	// Verify: Mapping deleted
	cachedVideoID, err = repo.GetChannelVideoMapping(ctx, channelID)
	require.NoError(t, err)
	assert.Empty(t, cachedVideoID, "Mapping should be deleted after offline")

	// Cleanup
	_ = repo.DeleteChannelVideoMapping(ctx, channelID)
}

// TestRepository_ChannelVideoMapping verifies Redis mapping CRUD operations
func TestRepository_ChannelVideoMapping(t *testing.T) {
	// Setup Redis test client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test DB
	})
	defer redisClient.Close()

	// Check Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for integration test")
	}

	logger := zap.NewNop()
	repo := NewRepository(redisClient, logger)

	channelID := "test_channel_crud"
	videoID := "test_video_456"

	// Test: Get non-existent mapping
	result, err := repo.GetChannelVideoMapping(ctx, channelID)
	require.NoError(t, err)
	assert.Empty(t, result, "Non-existent mapping should return empty string")

	// Test: Set mapping
	err = repo.SetChannelVideoMapping(ctx, channelID, videoID)
	require.NoError(t, err)

	// Test: Get existing mapping
	result, err = repo.GetChannelVideoMapping(ctx, channelID)
	require.NoError(t, err)
	assert.Equal(t, videoID, result)

	// Test: Delete mapping
	err = repo.DeleteChannelVideoMapping(ctx, channelID)
	require.NoError(t, err)

	// Test: Verify deletion
	result, err = repo.GetChannelVideoMapping(ctx, channelID)
	require.NoError(t, err)
	assert.Empty(t, result, "Deleted mapping should return empty string")

	// Cleanup
	_ = repo.DeleteChannelVideoMapping(ctx, channelID)
}

// TestStartDiscoveryLoop_Timeout verifies discovery loop gives up after max attempts
func TestStartDiscoveryLoop_Timeout(t *testing.T) {
	logger := zap.NewNop()

	// Mock discovery that always returns offline
	mockClient := &mockInnerTubeClient{}
	discovery := NewDiscovery(mockClient, logger)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Setup Redis (for caching discovered streams)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer redisClient.Close()

	// Check Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available for integration test")
	}

	repo := NewRepository(redisClient, logger)

	// Execute: Start discovery loop with timeout
	err := StartDiscoveryLoop(ctx, "test_channel_timeout", discovery, repo, logger)

	// Verify: Should timeout and return context error
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestStartDiscoveryLoop_Success verifies discovery loop succeeds when stream found
func TestStartDiscoveryLoop_Success(t *testing.T) {
	t.Skip("Skipping success test - requires mock discovery implementation")

	// Note: Full implementation requires mocking discovery.DiscoverStream
	// to return a video ID after N attempts. This is deferred to Phase 11
	// when stream discovery API integration is implemented.
}

// mockInnerTubeClient is a minimal mock for testing discovery
type mockInnerTubeClient struct{}

func (m *mockInnerTubeClient) GetLiveChatReplay(ctx context.Context, continuation string, visitorData string) (*innertube.LiveChatResponse, error) {
	return nil, nil
}

func (m *mockInnerTubeClient) ExtractContinuation(resp *innertube.LiveChatResponse) string {
	return ""
}

func (m *mockInnerTubeClient) GetPollInterval(resp *innertube.LiveChatResponse) time.Duration {
	return 2 * time.Second
}

// TestHandleStreamOffline_PublishesLifecycleEvent verifies that HandleStreamOffline
// publishes a JSON payload to Redis lifecycle:stream_end (EXPIRY-05).
func TestHandleStreamOffline_PublishesLifecycleEvent(t *testing.T) {
	type capturedPublish struct {
		channel string
		message string
	}
	var captured *capturedPublish

	mock := &mockLifecyclePublisher{
		publishFn: func(channel, message string) {
			captured = &capturedPublish{channel: channel, message: message}
		},
	}

	repo := &Repository{client: nil, logger: zap.NewNop()}
	// repo.DeleteChannelVideoMapping will fail with nil client — that's OK (non-fatal)
	err := HandleStreamOffline(context.Background(), "UC_channel_123", "video_456", repo, mock, zap.NewNop())
	assert.Error(t, err) // returns "stream offline" error

	require.NotNil(t, captured, "expected lifecycle event to be published")
	assert.Equal(t, "lifecycle:stream_end", captured.channel)
	assert.Contains(t, captured.message, "UC_channel_123")
	assert.Contains(t, captured.message, "youtube")
}

// mockLifecyclePublisher implements LifecyclePublisher for testing.
type mockLifecyclePublisher struct {
	publishFn func(channel, message string)
}

func (m *mockLifecyclePublisher) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	if m.publishFn != nil {
		m.publishFn(channel, fmt.Sprintf("%v", message))
	}
	return redis.NewIntCmd(ctx)
}
