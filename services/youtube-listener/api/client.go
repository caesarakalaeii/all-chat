package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		return true // Success - always charge
	}
	if isClientError(err) {
		return false // 4xx - don't charge
	}
	return true // 5xx, network errors, unknown - charge conservatively
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
	httpClient   *http.Client
	basePath     string
	userAgent    string
	quotaTracker *quota.Tracker
	logger       *zap.Logger
}

// NewClient creates a new YouTube API client wrapper
func NewClient(service *youtube.Service, httpClient *http.Client, quotaTracker *quota.Tracker, logger *zap.Logger) *Client {
	basePath := ""
	userAgent := ""
	if service != nil {
		basePath = service.BasePath
		userAgent = service.UserAgent
	}

	return &Client{
		service:      service,
		httpClient:   httpClient,
		basePath:     basePath,
		userAgent:    userAgent,
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

// GetChatMessages fetches messages from a live chat using streamList.
// This costs 5 quota units per call.
// Uses reserve-confirm-rollback pattern for accurate quota tracking.
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

	// STEP 2: Make YouTube API call (streamList)
	response, apiErr := c.getChatMessagesStream(ctx, liveChatID, pageToken)

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
	c.logAPICall(ctx, "LiveChatMessages.StreamList", chargedUnits, audit)

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

func (c *Client) getChatMessagesStream(ctx context.Context, liveChatID, pageToken string) (*youtube.LiveChatMessageListResponse, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("http client is required for live chat stream")
	}
	if c.basePath == "" {
		return nil, fmt.Errorf("youtube base path is missing")
	}

	params := url.Values{}
	params.Add("part", "id")
	params.Add("part", "snippet")
	params.Add("part", "authorDetails")
	params.Set("liveChatId", liveChatID)
	params.Set("maxResults", "2000")
	params.Set("prettyPrint", "false")
	params.Set("alt", "json")
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	endpoint := googleapi.ResolveRelative(c.basePath, "youtube/v3/liveChat/messages/stream")
	reqURL := endpoint + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	userAgent := googleapi.UserAgent
	if c.userAgent != "" {
		userAgent = userAgent + " " + c.userAgent
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := googleapi.CheckResponseWithBody(resp, body); err != nil {
		return nil, err
	}

	decoded, err := decodeLiveChatStreamResponse(body)
	if err != nil {
		return nil, err
	}
	decoded.ServerResponse = googleapi.ServerResponse{
		Header:         resp.Header,
		HTTPStatusCode: resp.StatusCode,
	}
	return decoded, nil
}

func decodeLiveChatStreamResponse(body []byte) (*youtube.LiveChatMessageListResponse, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("empty live chat stream response")
	}

	var single youtube.LiveChatMessageListResponse
	if err := json.Unmarshal(body, &single); err == nil {
		return &single, nil
	}

	var batch []youtube.LiveChatMessageListResponse
	if err := json.Unmarshal(body, &batch); err == nil {
		return mergeLiveChatResponses(batch), nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	for {
		var item youtube.LiveChatMessageListResponse
		if err := decoder.Decode(&item); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		batch = append(batch, item)
	}
	if len(batch) > 0 {
		return mergeLiveChatResponses(batch), nil
	}

	return nil, fmt.Errorf("unrecognized live chat stream response format")
}

func mergeLiveChatResponses(responses []youtube.LiveChatMessageListResponse) *youtube.LiveChatMessageListResponse {
	merged := &youtube.LiveChatMessageListResponse{}
	for _, response := range responses {
		if response.ActivePollItem != nil {
			merged.ActivePollItem = response.ActivePollItem
		}
		if response.Etag != "" {
			merged.Etag = response.Etag
		}
		if response.EventId != "" {
			merged.EventId = response.EventId
		}
		if response.Kind != "" {
			merged.Kind = response.Kind
		}
		if response.NextPageToken != "" {
			merged.NextPageToken = response.NextPageToken
		}
		if response.OfflineAt != "" {
			merged.OfflineAt = response.OfflineAt
		}
		if response.PageInfo != nil {
			merged.PageInfo = response.PageInfo
		}
		if response.PollingIntervalMillis != 0 {
			merged.PollingIntervalMillis = response.PollingIntervalMillis
		}
		if response.TokenPagination != nil {
			merged.TokenPagination = response.TokenPagination
		}
		if response.VisitorId != "" {
			merged.VisitorId = response.VisitorId
		}
		if len(response.Items) > 0 {
			merged.Items = append(merged.Items, response.Items...)
		}
	}
	return merged
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
