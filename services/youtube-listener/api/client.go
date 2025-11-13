package api

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"go.uber.org/zap"
	"google.golang.org/api/youtube/v3"
)

// Client wraps the YouTube API client with helper methods
type Client struct {
	service *youtube.Service
	logger  *zap.Logger
}

// NewClient creates a new YouTube API client wrapper
func NewClient(service *youtube.Service, logger *zap.Logger) *Client {
	return &Client{
		service: service,
		logger:  logger,
	}
}

// GetLiveStreams fetches active live streams for a channel
func (c *Client) GetLiveStreams(ctx context.Context, channelID string) ([]*models.YouTubeStream, error) {
	call := c.service.Search.List([]string{"id", "snippet"}).
		ChannelId(channelID).
		EventType("live").
		Type("video").
		MaxResults(5)

	response, err := call.Do()
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
		videoCall := c.service.Videos.List([]string{"liveStreamingDetails", "snippet"}).Id(videoID)
		videoResponse, err := videoCall.Do()
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
func (c *Client) GetChatMessages(ctx context.Context, liveChatID, pageToken string) (*youtube.LiveChatMessageListResponse, error) {
	call := c.service.LiveChatMessages.List(liveChatID, []string{"id", "snippet", "authorDetails"}).
		MaxResults(200)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	response, err := call.Do()
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
func (c *Client) CheckStreamStatus(ctx context.Context, videoID string) (bool, error) {
	call := c.service.Videos.List([]string{"liveStreamingDetails"}).Id(videoID)

	response, err := call.Do()
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
