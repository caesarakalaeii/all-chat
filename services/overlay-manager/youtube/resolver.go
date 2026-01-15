package youtube

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var (
	// ErrQuotaExhausted is returned when YouTube API quota is exhausted
	ErrQuotaExhausted = errors.New("youtube API quota exhausted")
)

var (
	// YouTube URL patterns
	videoURLPattern   = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`)
	channelURLPattern = regexp.MustCompile(`youtube\.com/channel/([a-zA-Z0-9_-]+)`)
	handlePattern     = regexp.MustCompile(`youtube\.com/@([a-zA-Z0-9_-]+)`)
	channelIDPattern  = regexp.MustCompile(`^UC[a-zA-Z0-9_-]{22}$`)
)

// Resolver resolves YouTube URLs/handles to channel IDs
type Resolver struct {
	apiKey      string
	httpClient  *http.Client
	quotaClient *clients.YouTubeQuotaClient
	logger      *zap.Logger
}

// NewResolver creates a new YouTube resolver
func NewResolver(apiKey string, quotaClient *clients.YouTubeQuotaClient, logger *zap.Logger) *Resolver {
	return &Resolver{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		quotaClient: quotaClient,
		logger:      logger,
	}
}

// ResolveToChannelID converts various YouTube inputs to a channel ID
func (r *Resolver) ResolveToChannelID(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)

	// Already a channel ID (starts with UC and is 24 chars)
	if channelIDPattern.MatchString(input) {
		return input, nil
	}

	// Extract from channel URL
	if matches := channelURLPattern.FindStringSubmatch(input); len(matches) > 1 {
		return matches[1], nil
	}

	// Extract from handle (@LofiGirl or youtube.com/@LofiGirl)
	var handle string
	if matches := handlePattern.FindStringSubmatch(input); len(matches) > 1 {
		handle = matches[1]
	} else if strings.HasPrefix(input, "@") {
		handle = strings.TrimPrefix(input, "@")
	}

	if handle != "" {
		return r.resolveHandleToChannelID(ctx, handle)
	}

	// Extract from video URL and get channel
	if matches := videoURLPattern.FindStringSubmatch(input); len(matches) > 1 {
		videoID := matches[1]
		return r.resolveVideoToChannelID(ctx, videoID)
	}

	return "", fmt.Errorf("unable to parse YouTube input: %s", input)
}

// resolveHandleToChannelID resolves a YouTube handle to channel ID using Search API
// Costs 100 quota units (Search.List)
func (r *Resolver) resolveHandleToChannelID(ctx context.Context, handle string) (string, error) {
	const quotaCost = 100  // Search.List API cost

	var reservationID string
	var response *youtube.SearchListResponse

	// 1. RESERVE QUOTA BEFORE API CALL (new reserve-confirm pattern)
	if r.quotaClient != nil {
		var err error
		reservationID, err = r.quotaClient.ReserveQuota(ctx, quotaCost, "search.list", false)
		if err != nil {
			r.logger.Warn("Failed to reserve quota for handle resolution",
				zap.String("handle", handle),
				zap.Int("units", quotaCost),
				zap.Error(err),
			)
			return "", ErrQuotaExhausted
		}

		// Ensure we confirm or rollback on return
		defer func() {
			// Determine if we should charge (rollback on 4xx client errors only)
			shouldCharge := response != nil || !isClientError(err)

			if confirmErr := r.quotaClient.ConfirmQuota(ctx, reservationID, quotaCost, shouldCharge); confirmErr != nil {
				r.logger.Warn("Failed to confirm/rollback quota reservation",
					zap.String("reservation_id", reservationID),
					zap.Bool("should_charge", shouldCharge),
					zap.Error(confirmErr),
				)
			}
		}()
	}

	// 2. MAKE YOUTUBE API CALL
	// FIX: Don't pass httpClient with API key - it interferes with authentication
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Search for channel by name/handle
	call := service.Search.List([]string{"snippet"}).
		Q(handle).
		Type("channel").
		MaxResults(1)

	response, err = call.Do()

	// 3. PROCESS RESPONSE
	if err != nil {
		return "", fmt.Errorf("failed to search for channel: %w", err)
	}

	if len(response.Items) == 0 {
		return "", fmt.Errorf("no channel found for handle: @%s", handle)
	}

	channelID := response.Items[0].Snippet.ChannelId
	if channelID == "" {
		return "", fmt.Errorf("no channel ID in search result")
	}

	return channelID, nil
}

// isClientError checks if an error is a 4xx client error (quota shouldn't be charged)
func isClientError(err error) bool {
	if err == nil {
		return false
	}
	// Check for 4xx status codes in error message
	// This is a simplified check - could be more robust
	errStr := err.Error()
	return strings.Contains(errStr, "400") || strings.Contains(errStr, "404") || strings.Contains(errStr, "403")
}

// resolveVideoToChannelID resolves a video ID to its channel ID
// Costs 1 quota unit (Videos.List)
func (r *Resolver) resolveVideoToChannelID(ctx context.Context, videoID string) (string, error) {
	const quotaCost = 1  // Videos.List API cost

	// DEBUG: Log API key status
	r.logger.Info("Resolving video to channel ID",
		zap.String("video_id", videoID),
		zap.Bool("has_api_key", r.apiKey != ""),
		zap.Int("api_key_length", len(r.apiKey)),
	)

	// 1. CHECK QUOTA BEFORE API CALL
	if r.quotaClient != nil {
		allowed, err := r.quotaClient.CheckQuota(ctx, quotaCost)
		if err != nil {
			r.logger.Warn("Failed to check quota (allowing request)", zap.Error(err))
		} else if !allowed {
			return "", ErrQuotaExhausted
		}
	}

	// 2. MAKE YOUTUBE API CALL
	// FIX: Don't pass httpClient with API key - it interferes with authentication
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Get video details
	call := service.Videos.List([]string{"snippet"}).Id(videoID)
	response, err := call.Do()

	// 3. RECORD USAGE AFTER API CALL
	if r.quotaClient != nil {
		if recordErr := r.quotaClient.RecordUsage(ctx, quotaCost); recordErr != nil {
			r.logger.Warn("Failed to record quota usage",
				zap.Int("units", quotaCost),
				zap.Error(recordErr),
			)
		}
	}

	// 4. PROCESS RESPONSE
	if err != nil {
		return "", fmt.Errorf("failed to get video details: %w", err)
	}

	if len(response.Items) == 0 {
		return "", fmt.Errorf("video not found: %s", videoID)
	}

	channelID := response.Items[0].Snippet.ChannelId
	if channelID == "" {
		return "", fmt.Errorf("no channel ID in video details")
	}

	return channelID, nil
}

// GetChannelInfo gets channel information for display
// Costs 1 quota unit (Channels.List)
func (r *Resolver) GetChannelInfo(ctx context.Context, channelID string) (*ChannelInfo, error) {
	const quotaCost = 1  // Channels.List API cost

	// 1. CHECK QUOTA BEFORE API CALL
	if r.quotaClient != nil {
		allowed, err := r.quotaClient.CheckQuota(ctx, quotaCost)
		if err != nil {
			r.logger.Warn("Failed to check quota (allowing request)", zap.Error(err))
		} else if !allowed {
			return nil, ErrQuotaExhausted
		}
	}

	// 2. MAKE YOUTUBE API CALL
	// FIX: Don't pass httpClient with API key - it interferes with authentication
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	call := service.Channels.List([]string{"snippet", "statistics"}).Id(channelID)
	response, err := call.Do()

	// 3. RECORD USAGE AFTER API CALL
	if r.quotaClient != nil {
		if recordErr := r.quotaClient.RecordUsage(ctx, quotaCost); recordErr != nil {
			r.logger.Warn("Failed to record quota usage",
				zap.Int("units", quotaCost),
				zap.Error(recordErr),
			)
		}
	}

	// 4. PROCESS RESPONSE
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}

	channel := response.Items[0]
	return &ChannelInfo{
		ChannelID:   channelID,
		Title:       channel.Snippet.Title,
		Description:  channel.Snippet.Description,
		CustomURL:   channel.Snippet.CustomUrl,
		Thumbnail:   channel.Snippet.Thumbnails.Default.Url,
	}, nil
}

// ChannelInfo contains YouTube channel information
type ChannelInfo struct {
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CustomURL   string `json:"custom_url"`
	Thumbnail   string `json:"thumbnail"`
}
