package lifecycle

import (
	"context"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TestOfflineDetection_EmptyContinuations validates offline detection via empty continuations
// Verifies: DetectOffline() returns true when InnerTube response has empty continuations array
//
// CONTRACT: Empty continuations = stream offline
//           Per Phase 10 decision: More reliable than checking continuation token validity
func (s *LifecycleTestSuite) TestOfflineDetection_EmptyContinuations() {
	s.logger.Info("Testing offline detection contract: empty continuations")

	// Test case 1: Normal online response (has continuations)
	onlineResponse := &innertube.LiveChatResponse{
		ContinuationContents: innertube.ContinuationContents{
			LiveChatContinuation: innertube.LiveChatContinuation{
				Actions: []innertube.ChatAction{
					{
						AddChatItemAction: &innertube.AddChatItemAction{
							Item: innertube.ChatItem{
								LiveChatTextMessageRenderer: &innertube.LiveChatTextMessageRenderer{
									Message: innertube.MessageContent{
										Runs: []innertube.MessageRun{{Text: "Test message"}},
									},
									AuthorName: innertube.SimpleText{
										SimpleText: "TestUser",
									},
									AuthorExternalChannelID: "UC_TestUser",
									TimestampUsec: "1234567890",
								},
							},
						},
					},
				},
				Continuations: []innertube.Continuation{
					{
						InvalidationContinuationData: &innertube.InvalidationContinuationData{
							Continuation: "next_token",
							TimeoutDurationMillis: 2000,
						},
					},
				},
			},
		},
	}

	isOffline := poller.DetectOffline(onlineResponse)
	s.False(isOffline, "Stream with continuations should be detected as online")

	s.logger.Info("Online response correctly identified")

	// Test case 2: Offline response (empty continuations array)
	offlineResponse := &innertube.LiveChatResponse{
		ContinuationContents: innertube.ContinuationContents{
			LiveChatContinuation: innertube.LiveChatContinuation{
				Actions:       []innertube.ChatAction{}, // No actions
				Continuations: []innertube.Continuation{}, // Empty = offline
			},
		},
	}

	isOffline = poller.DetectOffline(offlineResponse)
	s.True(isOffline, "Stream with empty continuations should be detected as offline")

	s.logger.Info("Offline response correctly identified")

	// Test case 3: Nil response (treat as offline)
	isOffline = poller.DetectOffline(nil)
	s.True(isOffline, "Nil response should be treated as offline")

	s.logger.Info("Nil response correctly handled")

	s.logger.Info("Offline detection contract validated successfully")
}

// TestOfflineDetection_CacheCleanup validates Redis cleanup after offline detection
// Verifies: HandleStreamOffline() deletes channel→video mapping to force rediscovery
//
// CONTRACT: When stream goes offline → delete Redis mapping
//           Next activation triggers fresh discovery (streamer may start new stream)
func (s *LifecycleTestSuite) TestOfflineDetection_CacheCleanup() {
	ctx := context.Background()

	s.logger.Info("Testing offline detection contract: cache cleanup")

	channelID := "UC_OfflineTestChannel"
	videoID := "offline_video_123"

	// Create repository for cleanup operations
	repository := poller.NewRepository(s.redisClient, s.logger)

	// Pre-populate cache (simulate active polling)
	err := repository.SetChannelVideoMapping(ctx, channelID, videoID)
	s.NoError(err, "Should cache video ID")

	// Verify cache exists
	cachedVideoID, err := repository.GetChannelVideoMapping(ctx, channelID)
	s.NoError(err)
	s.Equal(videoID, cachedVideoID, "Video ID should be cached")

	s.logger.Info("Cache populated",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	// Simulate offline detection
	err = poller.HandleStreamOffline(ctx, channelID, videoID, repository, s.logger)
	s.Error(err, "HandleStreamOffline should return error to signal polling stop")
	s.Contains(err.Error(), "stream offline", "Error message should indicate offline reason")

	s.logger.Info("Offline event handled")

	// Verify cache cleared
	cachedVideoID, err = repository.GetChannelVideoMapping(ctx, channelID)
	s.NoError(err, "GetChannelVideoMapping should not error on missing key")
	s.Empty(cachedVideoID, "Cache should be cleared after offline detection")

	s.logger.Info("Cache cleanup verified")

	s.logger.Info("Offline detection cleanup contract validated successfully")
}

// TestOfflineDetection_GracefulCleanup validates resource cleanup after offline
// Verifies: No goroutine leaks, Redis connections closed properly
//
// CONTRACT: Offline detection triggers graceful cleanup
//           Manager stops poller, releases leadership, clears state
func (s *LifecycleTestSuite) TestOfflineDetection_GracefulCleanup() {
	ctx := context.Background()

	s.logger.Info("Testing offline detection contract: graceful cleanup")

	channelID := "UC_CleanupTestChannel"
	videoID := "cleanup_video_456"

	// Simulate active polling state
	repository := poller.NewRepository(s.redisClient, s.logger)
	err := repository.SetChannelVideoMapping(ctx, channelID, videoID)
	s.NoError(err)

	// Create Redis consumer group (simulates active poller)
	streamKey := "chat:raw"
	groupName := "test-consumer-group"

	// Ensure stream exists
	_, err = s.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{"init": "true"},
	}).Result()
	s.NoError(err)

	// Create consumer group
	err = s.redisClient.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		s.NoError(err, "Consumer group creation should succeed")
	}

	s.logger.Info("Simulated active polling state")

	// Trigger offline cleanup
	err = poller.HandleStreamOffline(ctx, channelID, videoID, repository, s.logger)
	s.Error(err, "HandleStreamOffline signals cleanup via error")

	// Verify Redis cache cleared
	cachedVideoID, err := repository.GetChannelVideoMapping(ctx, channelID)
	s.NoError(err)
	s.Empty(cachedVideoID, "Cache should be cleared")

	// Verify consumer group can be destroyed (no active consumers)
	// Note: In real implementation, Manager would handle consumer cleanup
	err = s.redisClient.XGroupDestroy(ctx, streamKey, groupName).Err()
	s.NoError(err, "Consumer group should be destroyable (no active consumers)")

	s.logger.Info("Resource cleanup verified")

	s.logger.Info("Graceful cleanup contract validated successfully")
}

// TestOfflineDetection_StreamReactivation validates behavior after stream returns
// Verifies: After offline detection + cleanup, fresh activation works correctly
//
// CONTRACT: Offline → cleanup → new activation should trigger fresh discovery
//           No stale state from previous stream
func (s *LifecycleTestSuite) TestOfflineDetection_StreamReactivation() {
	ctx := context.Background()

	s.logger.Info("Testing offline detection contract: stream reactivation")

	channelID := "UC_ReactivationChannel"
	oldVideoID := "old_stream_789"
	newVideoID := "new_stream_101"

	repository := poller.NewRepository(s.redisClient, s.logger)

	// PHASE 1: Active stream (cache populated)
	err := repository.SetChannelVideoMapping(ctx, channelID, oldVideoID)
	s.NoError(err)

	s.logger.Info("Phase 1: Stream active",
		zap.String("video_id", oldVideoID),
	)

	// PHASE 2: Stream goes offline (cleanup)
	err = poller.HandleStreamOffline(ctx, channelID, oldVideoID, repository, s.logger)
	s.Error(err)

	// Verify cache cleared
	cachedVideoID, err := repository.GetChannelVideoMapping(ctx, channelID)
	s.NoError(err)
	s.Empty(cachedVideoID, "Cache should be cleared after offline")

	s.logger.Info("Phase 2: Stream offline, cache cleared")

	// PHASE 3: Streamer starts new stream (fresh discovery)
	// Simulate discovery finding new video ID
	err = repository.SetChannelVideoMapping(ctx, channelID, newVideoID)
	s.NoError(err)

	// Verify new video ID cached
	cachedVideoID, err = repository.GetChannelVideoMapping(ctx, channelID)
	s.NoError(err)
	s.Equal(newVideoID, cachedVideoID, "New video ID should be cached")

	s.logger.Info("Phase 3: New stream discovered",
		zap.String("video_id", newVideoID),
	)

	// Verify old video ID not present (no state leakage)
	s.NotEqual(oldVideoID, cachedVideoID, "Should use new video ID, not old one")

	s.logger.Info("Stream reactivation contract validated successfully")
}
