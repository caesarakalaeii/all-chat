package api

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
	"google.golang.org/api/youtube/v3"
)

// Client wraps the YouTube API client with helper methods
type Client struct {
	service      *youtube.Service
	quotaTracker *quota.Tracker
	logger       *zap.Logger
}

// NewClient creates a new YouTube API client wrapper
func NewClient(service *youtube.Service, quotaTracker *quota.Tracker, logger *zap.Logger) *Client {
	return &Client{
		service:      service,
		quotaTracker: quotaTracker,
		logger:       logger,
	}
}

// GetLiveStreams fetches active live streams for a channel
func (c *Client) GetLiveStreams(ctx context.Context, channelID string) ([]*models.YouTubeStream, error) {
	// Check quota BEFORE making expensive search.list call (100 units)
	if c.quotaTracker != nil && !c.quotaTracker.CanMakeRequest(quota.QuotaCostSearch) {
		return nil, fmt.Errorf("insufficient quota: %d units remaining, need %d for search", 
			c.quotaTracker.GetRemainingQuota(), quota.QuotaCostSearch)
	}

	call := c.service.Search.List([]string{"id", "snippet"}).
		ChannelId(channelID).
		EventType("live").
		Type("video").
		MaxResults(5)

	response, err := call.Do()

	// Record quota usage for search.list (100 units)
	if c.quotaTracker != nil {
		if recordErr := c.quotaTracker.RecordUsage(ctx, quota.QuotaCostSearch); recordErr != nil {
			c.logger.Warn("Failed to record search.list quota usage", zap.Error(recordErr))
		}
	}

	if err != nil {
		c.logger.Error("Failed to fetch live streams",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to fetch live streams: %w", err)
	}

	streams := make([]*models.YouTubeStream, 0, len(response.Items))

	for _, item := range response.Items {
		if item.Id == nil || item.Id.VideoId == "" {
			continue
		}

		// Get live streaming details including liveChatId
		videoID := item.Id.VideoId
		
		// Check quota BEFORE making videos.list call (1 unit)
		if c.quotaTracker != nil && !c.quotaTracker.CanMakeRequest(quota.QuotaCostVideos) {
			c.logger.Warn("Insufficient quota for videos.list call, stopping stream enumeration",
				zap.String("video_id", videoID),
				zap.Int("remaining_quota", c.quotaTracker.GetRemainingQuota()),
			)
			break
		}

		videoCall := c.service.Videos.List([]string{"liveStreamingDetails", "snippet"}).Id(videoID)
		videoResponse, err := videoCall.Do()

		// Record quota usage for videos.list (1 unit)
		if c.quotaTracker != nil {
			if recordErr := c.quotaTracker.RecordUsage(ctx, quota.QuotaCostVideos); recordErr != nil {
				c.logger.Warn("Failed to record videos.list quota usage", zap.Error(recordErr))
			}
		}

		if err != nil {
			c.logger.Warn("Failed to get video details",
				zap.String("video_id", videoID),
				zap.Error(err),
			)
			continue
		}

		if len(videoResponse.Items) == 0 {
			continue
		}

		video := videoResponse.Items[0]
		if video.LiveStreamingDetails == nil || video.LiveStreamingDetails.ActiveLiveChatId == "" {
			c.logger.Debug("No active live chat for video",
				zap.String("video_id", videoID),
			)
			continue
		}

		stream := &models.YouTubeStream{
			StreamID:        videoID,
			ChannelID:       channelID,
			ChannelName:     video.Snippet.ChannelTitle,
			LiveChatID:      video.LiveStreamingDetails.ActiveLiveChatId,
			IsLive:          true,
			PollingInterval: 5000, // Default 5 seconds, will be updated from API
			NextPageToken:   "",
			LastPolledAt:    time.Time{},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		streams = append(streams, stream)
	}

	c.logger.Info("Fetched live streams",
		zap.String("channel_id", channelID),
		zap.Int("count", len(streams)),
	)

	return streams, nil
}

// GetChatMessages fetches messages from a live chat
// This costs 5 quota units per call
func (c *Client) GetChatMessages(ctx context.Context, liveChatID, pageToken string) (*youtube.LiveChatMessageListResponse, error) {
	// Check quota BEFORE making the API call
	if c.quotaTracker != nil && !c.quotaTracker.CanMakeRequest(quota.QuotaCostLiveChatMessages) {
		return nil, fmt.Errorf("insufficient quota: %d units remaining, need %d", 
			c.quotaTracker.GetRemainingQuota(), quota.QuotaCostLiveChatMessages)
	}

	call := c.service.LiveChatMessages.List(liveChatID, []string{"id", "snippet", "authorDetails"}).
		MaxResults(200)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	response, err := call.Do()

	// Record quota usage AFTER the API call (whether success or failure)
	if c.quotaTracker != nil {
		if recordErr := c.quotaTracker.RecordUsage(ctx, quota.QuotaCostLiveChatMessages); recordErr != nil {
			c.logger.Warn("Failed to record liveChatMessages.list quota usage", zap.Error(recordErr))
		}
	}

	if err != nil {
		c.logger.Error("Failed to fetch chat messages",
			zap.String("live_chat_id", liveChatID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to fetch chat messages: %w", err)
	}

	c.logger.Debug("Fetched chat messages",
		zap.String("live_chat_id", liveChatID),
		zap.Int("count", len(response.Items)),
		zap.Int("polling_interval", int(response.PollingIntervalMillis)),
	)

	return response, nil
}

// CheckStreamStatus checks if a stream is still live
// This is a lightweight check that costs 1 quota unit
func (c *Client) CheckStreamStatus(ctx context.Context, videoID string) (bool, error) {
	// Check quota BEFORE making the API call
	if c.quotaTracker != nil && !c.quotaTracker.CanMakeRequest(quota.QuotaCostVideos) {
		return false, fmt.Errorf("insufficient quota: %d units remaining, need %d", 
			c.quotaTracker.GetRemainingQuota(), quota.QuotaCostVideos)
	}

	call := c.service.Videos.List([]string{"liveStreamingDetails"}).Id(videoID)

	response, err := call.Do()

	// Record quota usage for videos.list (1 unit)
	if c.quotaTracker != nil {
		if recordErr := c.quotaTracker.RecordUsage(ctx, quota.QuotaCostVideos); recordErr != nil {
			c.logger.Warn("Failed to record videos.list quota usage", zap.Error(recordErr))
		}
	}

	if err != nil {
		c.logger.Error("Failed to check stream status",
			zap.String("video_id", videoID),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to check stream status: %w", err)
	}

	if len(response.Items) == 0 {
		return false, nil
	}

	video := response.Items[0]
	if video.LiveStreamingDetails == nil {
		return false, nil
	}

	// Stream is live if it has started and not ended
	isLive := video.LiveStreamingDetails.ActualStartTime != "" &&
		video.LiveStreamingDetails.ActualEndTime == ""

	return isLive, nil
}

// GetVideoDetails fetches detailed information about a video
// This costs 1 quota unit and is used after status check confirms stream is live
func (c *Client) GetVideoDetails(ctx context.Context, videoID string) (*models.YouTubeStream, error) {
	// Check quota BEFORE making the API call
	if c.quotaTracker != nil && !c.quotaTracker.CanMakeRequest(quota.QuotaCostVideos) {
		return nil, fmt.Errorf("insufficient quota: %d units remaining, need %d", 
			c.quotaTracker.GetRemainingQuota(), quota.QuotaCostVideos)
	}

	call := c.service.Videos.List([]string{"snippet", "liveStreamingDetails"}).Id(videoID)

	response, err := call.Do()

	// Record quota usage for videos.list (1 unit)
	if c.quotaTracker != nil {
		if recordErr := c.quotaTracker.RecordUsage(ctx, quota.QuotaCostVideos); recordErr != nil {
			c.logger.Warn("Failed to record videos.list quota usage", zap.Error(recordErr))
		}
	}

	if err != nil {
		c.logger.Error("Failed to get video details",
			zap.String("video_id", videoID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get video details: %w", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}

	video := response.Items[0]

	// Extract basic info
	stream := &models.YouTubeStream{
		VideoID:         videoID,
		ChannelID:       video.Snippet.ChannelId,
		ChannelName:     video.Snippet.ChannelTitle,
		Title:           video.Snippet.Title,
		IsLive:          false,
		PollingInterval: 3000, // Default 3 seconds, will be updated from API response
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Add thumbnail URL if available
	if video.Snippet.Thumbnails != nil {
		if video.Snippet.Thumbnails.Medium != nil {
			stream.ThumbnailURL = video.Snippet.Thumbnails.Medium.Url
		} else if video.Snippet.Thumbnails.Default != nil {
			stream.ThumbnailURL = video.Snippet.Thumbnails.Default.Url
		}
	}

	// Add live streaming details if available
	if video.LiveStreamingDetails != nil {
		stream.IsLive = video.LiveStreamingDetails.ActualStartTime != "" &&
			video.LiveStreamingDetails.ActualEndTime == ""

		if video.LiveStreamingDetails.ActiveLiveChatId != "" {
			stream.LiveChatID = video.LiveStreamingDetails.ActiveLiveChatId
		}

		// Parse published date if available
		if video.Snippet.PublishedAt != "" {
			if publishedAt, parseErr := time.Parse(time.RFC3339, video.Snippet.PublishedAt); parseErr == nil {
				stream.PublishedAt = publishedAt
			}
		}
	}

	c.logger.Debug("Fetched video details",
		zap.String("video_id", videoID),
		zap.String("title", stream.Title),
		zap.Bool("is_live", stream.IsLive),
	)

	return stream, nil
}
