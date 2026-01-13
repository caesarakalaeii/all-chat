package api

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// isClientError checks if error is a 4xx client error that didn't reach YouTube
// These errors should NOT be charged quota (e.g., invalid request format)
func isClientError(err error) bool {
	if apiErr, ok := err.(*googleapi.Error); ok {
		return apiErr.Code >= 400 && apiErr.Code < 500
	}
	return false
}

// shouldChargeQuota determines if we should charge quota for this error
// Returns true for: nil (success), 5xx server errors, network errors
// Returns false for: 4xx client errors (didn't reach YouTube properly)
func shouldChargeQuota(err error) bool {
	if err == nil {
		return true  // Success - always charge
	}
	if isClientError(err) {
		return false  // 4xx - don't charge
	}
	return true  // 5xx, network errors, unknown - charge conservatively
}

// StreamStatusResult contains stream status and details from a single API call
// This eliminates the need for a separate GetVideoDetails call (saves 1 quota unit)
type StreamStatusResult struct {
	IsLive      bool
	LiveChatID  string
	Title       string
	ChannelName string
}

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

func (c *Client) logAPICall(ctx context.Context, endpoint string, units int, audit *quota.AuditContext) {
	if c.quotaTracker == nil {
		return
	}

	c.quotaTracker.LogAPICall(ctx, endpoint, units, audit)
}

// GetLiveStreams fetches active live streams for a channel
// Costs 100 units for search + 1 unit per video found
// Uses reserve-confirm-rollback pattern for accurate quota tracking
func (c *Client) GetLiveStreams(ctx context.Context, channelID string, audit *quota.AuditContext) ([]*models.YouTubeStream, error) {
	const searchCost = quota.QuotaCostSearch
	if audit == nil {
		audit = &quota.AuditContext{ChannelID: channelID}
	} else if audit.ChannelID == "" {
		audit.ChannelID = channelID
	}

	// STEP 1: RESERVE quota for search.list (100 units)
	var searchReservationID string
	var err error
	if c.quotaTracker != nil {
		searchReservationID, err = c.quotaTracker.ReserveQuota(ctx, searchCost)
		if err != nil {
			return nil, fmt.Errorf("insufficient quota for search: %w", err)
		}
	}

	// STEP 2: Make Search.List API call
	call := c.service.Search.List([]string{"id", "snippet"}).
		ChannelId(channelID).
		EventType("live").
		Type("video").
		MaxResults(5)

	response, apiErr := call.Do()

	// STEP 3: CONFIRM or ROLLBACK search.list quota
	chargedUnits := 0
	if c.quotaTracker != nil && searchReservationID != "" {
		if shouldChargeQuota(apiErr) {
			if confirmErr := c.quotaTracker.ConfirmReservation(ctx, searchReservationID, searchCost); confirmErr != nil {
				c.logger.Warn("Failed to confirm search quota reservation", zap.Error(confirmErr))
			} else {
				chargedUnits = searchCost
			}
		} else {
			if rollbackErr := c.quotaTracker.RollbackReservation(ctx, searchReservationID, searchCost); rollbackErr != nil {
				c.logger.Warn("Failed to rollback search quota reservation", zap.Error(rollbackErr))
			}
		}
	}
	c.logAPICall(ctx, "Search.List", chargedUnits, audit)

	// STEP 4: Check for search errors
	if apiErr != nil {
		c.logger.Error("Failed to fetch live streams",
			zap.String("channel_id", channelID),
			zap.Bool("charged_quota", shouldChargeQuota(apiErr)),
			zap.Error(apiErr),
		)
		return nil, fmt.Errorf("failed to fetch live streams: %w", apiErr)
	}

	// STEP 5: Process each video found (each costs 1 unit)
	streams := make([]*models.YouTubeStream, 0, len(response.Items))

	for _, item := range response.Items {
		if item.Id == nil || item.Id.VideoId == "" {
			continue
		}

		videoID := item.Id.VideoId
		const videoCost = quota.QuotaCostVideos

		// Reserve quota for videos.list
		var videoReservationID string
		if c.quotaTracker != nil {
			videoReservationID, err = c.quotaTracker.ReserveQuota(ctx, videoCost)
			if err != nil {
				c.logger.Warn("Insufficient quota for videos.list, stopping enumeration",
					zap.String("video_id", videoID),
				)
				break
			}
		}

		// Make videos.list call
		videoCall := c.service.Videos.List([]string{"liveStreamingDetails", "snippet"}).Id(videoID)
		videoResponse, videoErr := videoCall.Do()

		// Confirm or rollback videos.list quota
		videoAudit := &quota.AuditContext{
			ChannelID: channelID,
			VideoID:   videoID,
			OverlayID: audit.OverlayID,
		}
		videoChargedUnits := 0
		if c.quotaTracker != nil && videoReservationID != "" {
			if shouldChargeQuota(videoErr) {
				if confirmErr := c.quotaTracker.ConfirmReservation(ctx, videoReservationID, videoCost); confirmErr != nil {
					c.logger.Warn("Failed to confirm video quota reservation", zap.Error(confirmErr))
				} else {
					videoChargedUnits = videoCost
				}
			} else {
				if rollbackErr := c.quotaTracker.RollbackReservation(ctx, videoReservationID, videoCost); rollbackErr != nil {
					c.logger.Warn("Failed to rollback video quota reservation", zap.Error(rollbackErr))
				}
			}
		}
		c.logAPICall(ctx, "Videos.List", videoChargedUnits, videoAudit)

		if videoErr != nil {
			c.logger.Warn("Failed to get video details",
				zap.String("video_id", videoID),
				zap.Bool("charged_quota", shouldChargeQuota(videoErr)),
				zap.Error(videoErr),
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
// Uses reserve-confirm-rollback pattern for accurate quota tracking
func (c *Client) GetChatMessages(ctx context.Context, liveChatID, pageToken string, audit *quota.AuditContext) (*youtube.LiveChatMessageListResponse, error) {
	const cost = quota.QuotaCostLiveChatMessages

	// STEP 1: RESERVE quota BEFORE making API call
	var reservationID string
	var err error
	if c.quotaTracker != nil {
		reservationID, err = c.quotaTracker.ReserveQuota(ctx, cost)
		if err != nil {
			return nil, fmt.Errorf("insufficient quota: %w", err)
		}
	}

	// STEP 2: Make YouTube API call
	call := c.service.LiveChatMessages.List(liveChatID, []string{"id", "snippet", "authorDetails"}).
		MaxResults(200)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	response, apiErr := call.Do()

	// STEP 3: CONFIRM or ROLLBACK based on result
	chargedUnits := 0
	if c.quotaTracker != nil && reservationID != "" {
		if shouldChargeQuota(apiErr) {
			// API succeeded or failed with server error - confirm quota usage
			if confirmErr := c.quotaTracker.ConfirmReservation(ctx, reservationID, cost); confirmErr != nil {
				c.logger.Warn("Failed to confirm quota reservation", zap.Error(confirmErr))
			} else {
				chargedUnits = cost
			}
		} else {
			// Client error (4xx) - rollback quota
			if rollbackErr := c.quotaTracker.RollbackReservation(ctx, reservationID, cost); rollbackErr != nil {
				c.logger.Warn("Failed to rollback quota reservation", zap.Error(rollbackErr))
			}
		}
	}
	c.logAPICall(ctx, "LiveChatMessages.List", chargedUnits, audit)

	// STEP 4: Process response
	if apiErr != nil {
		c.logger.Error("Failed to fetch chat messages",
			zap.String("live_chat_id", liveChatID),
			zap.Bool("charged_quota", shouldChargeQuota(apiErr)),
			zap.Error(apiErr),
		)
		return nil, fmt.Errorf("failed to fetch chat messages: %w", apiErr)
	}

	c.logger.Debug("Fetched chat messages",
		zap.String("live_chat_id", liveChatID),
		zap.Int("count", len(response.Items)),
		zap.Int("polling_interval", int(response.PollingIntervalMillis)),
	)

	return response, nil
}

// CheckStreamStatus checks if a stream is still live and returns full details
// This costs 1 quota unit and includes all data needed to start polling
// Eliminates the need for a separate GetVideoDetails call (saves 1 unit per check)
// Uses reserve-confirm-rollback pattern for accurate quota tracking
func (c *Client) CheckStreamStatus(ctx context.Context, videoID string, audit *quota.AuditContext) (*StreamStatusResult, error) {
	const cost = quota.QuotaCostVideos
	if audit == nil {
		audit = &quota.AuditContext{VideoID: videoID}
	} else if audit.VideoID == "" {
		audit.VideoID = videoID
	}

	// STEP 1: RESERVE quota BEFORE making API call
	var reservationID string
	var err error
	if c.quotaTracker != nil {
		reservationID, err = c.quotaTracker.ReserveQuota(ctx, cost)
		if err != nil {
			return nil, fmt.Errorf("insufficient quota: %w", err)
		}
	}

	// STEP 2: Make YouTube API call
	// Request both liveStreamingDetails AND snippet in a single call
	call := c.service.Videos.List([]string{"liveStreamingDetails", "snippet"}).Id(videoID)
	response, apiErr := call.Do()

	// STEP 3: CONFIRM or ROLLBACK based on result
	chargedUnits := 0
	if c.quotaTracker != nil && reservationID != "" {
		if shouldChargeQuota(apiErr) {
			if confirmErr := c.quotaTracker.ConfirmReservation(ctx, reservationID, cost); confirmErr != nil {
				c.logger.Warn("Failed to confirm quota reservation", zap.Error(confirmErr))
			} else {
				chargedUnits = cost
			}
		} else {
			if rollbackErr := c.quotaTracker.RollbackReservation(ctx, reservationID, cost); rollbackErr != nil {
				c.logger.Warn("Failed to rollback quota reservation", zap.Error(rollbackErr))
			}
		}
	}
	c.logAPICall(ctx, "Videos.List", chargedUnits, audit)

	// STEP 4: Process response
	if apiErr != nil {
		c.logger.Error("Failed to check stream status",
			zap.String("video_id", videoID),
			zap.Bool("charged_quota", shouldChargeQuota(apiErr)),
			zap.Error(apiErr),
		)
		return nil, fmt.Errorf("failed to check stream status: %w", apiErr)
	}

	if len(response.Items) == 0 {
		return &StreamStatusResult{IsLive: false}, nil
	}

	video := response.Items[0]
	if video.LiveStreamingDetails == nil {
		return &StreamStatusResult{IsLive: false}, nil
	}

	// Stream is live if it has started and not ended
	isLive := video.LiveStreamingDetails.ActualStartTime != "" &&
		video.LiveStreamingDetails.ActualEndTime == ""

	// Return all needed details in a single struct
	result := &StreamStatusResult{
		IsLive:      isLive,
		LiveChatID:  video.LiveStreamingDetails.ActiveLiveChatId,
		Title:       video.Snippet.Title,
		ChannelName: video.Snippet.ChannelTitle,
	}

	return result, nil
}

// GetVideoDetails fetches detailed information about a video
// This costs 1 quota unit and is used after status check confirms stream is live
func (c *Client) GetVideoDetails(ctx context.Context, videoID string, audit *quota.AuditContext) (*models.YouTubeStream, error) {
	// Check quota BEFORE making the API call
	if c.quotaTracker != nil && !c.quotaTracker.CanMakeRequest(quota.QuotaCostVideos) {
		return nil, fmt.Errorf("insufficient quota: %d units remaining, need %d", 
			c.quotaTracker.GetRemainingQuota(), quota.QuotaCostVideos)
	}
	if audit == nil {
		audit = &quota.AuditContext{VideoID: videoID}
	} else if audit.VideoID == "" {
		audit.VideoID = videoID
	}

	call := c.service.Videos.List([]string{"snippet", "liveStreamingDetails"}).Id(videoID)

	response, err := call.Do()

	chargedUnits := 0
	// Record quota usage for videos.list (1 unit)
	if c.quotaTracker != nil {
		if recordErr := c.quotaTracker.RecordUsage(ctx, quota.QuotaCostVideos); recordErr != nil {
			c.logger.Warn("Failed to record videos.list quota usage", zap.Error(recordErr))
		} else {
			chargedUnits = quota.QuotaCostVideos
		}
	}
	c.logAPICall(ctx, "Videos.List", chargedUnits, audit)

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
