package youtube

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
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
	apiKey     string
	httpClient *http.Client
}

// NewResolver creates a new YouTube resolver
func NewResolver(apiKey string) *Resolver {
	return &Resolver{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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
func (r *Resolver) resolveHandleToChannelID(ctx context.Context, handle string) (string, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.apiKey), option.WithHTTPClient(r.httpClient))
	if err != nil {
		return "", fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Search for channel by name/handle
	call := service.Search.List([]string{"snippet"}).
		Q(handle).
		Type("channel").
		MaxResults(1)

	response, err := call.Do()
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

// resolveVideoToChannelID resolves a video ID to its channel ID
func (r *Resolver) resolveVideoToChannelID(ctx context.Context, videoID string) (string, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.apiKey), option.WithHTTPClient(r.httpClient))
	if err != nil {
		return "", fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Get video details
	call := service.Videos.List([]string{"snippet"}).Id(videoID)
	response, err := call.Do()
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
func (r *Resolver) GetChannelInfo(ctx context.Context, channelID string) (*ChannelInfo, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.apiKey), option.WithHTTPClient(r.httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	call := service.Channels.List([]string{"snippet", "statistics"}).Id(channelID)
	response, err := call.Do()
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
		Description: channel.Snippet.Description,
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
