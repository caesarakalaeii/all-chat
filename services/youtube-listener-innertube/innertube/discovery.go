package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

// Discovery handles YouTube live stream discovery from channel pages
type Discovery struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewDiscovery creates a new Discovery instance
func NewDiscovery(httpClient *http.Client, logger *zap.Logger) *Discovery {
	return &Discovery{
		httpClient: httpClient,
		logger:     logger,
	}
}

// DiscoverLiveStream discovers the live video ID for a given channel ID using InnerTube Browse API
// Returns the video ID if a live stream is found, or an error if none exists
func (d *Discovery) DiscoverLiveStream(ctx context.Context, channelID string) (string, error) {
	d.logger.Info("discovering live stream",
		zap.String("channel_id", channelID),
	)

	// Use InnerTube Browse API to get channel's live tab content
	url := fmt.Sprintf("https://www.youtube.com/youtubei/v1/browse?key=%s", DefaultAPIKey)

	// Build InnerTube Browse request for channel's live tab
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": DefaultClientVersion,
			},
		},
		"browseId": channelID,
		"params":   "EgdzdHJlYW1z8gYECgJ6AA%3D%3D", // "streams" tab encoded params
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Error("failed to fetch channel browse data",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return "", fmt.Errorf("fetch channel browse data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("unexpected status code from browse API",
			zap.String("channel_id", channelID),
			zap.Int("status_code", resp.StatusCode),
		)
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	// Parse JSON response
	var browseResponse map[string]interface{}
	if err := json.Unmarshal(body, &browseResponse); err != nil {
		return "", fmt.Errorf("parse browse response: %w", err)
	}

	// Extract video ID from browse response
	// Look for richItemRenderer with videoId in the live tab
	videoID := extractVideoIDFromBrowse(browseResponse)
	if videoID == "" {
		d.logger.Info("no live stream found",
			zap.String("channel_id", channelID),
		)
		return "", fmt.Errorf("no live stream found for channel %s", channelID)
	}

	d.logger.Info("discovered live stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	return videoID, nil
}

// extractVideoIDFromBrowse recursively searches the InnerTube Browse API response for a video ID
// Looks for richItemRenderer with videoId or videoRenderer with videoId in live tab
func extractVideoIDFromBrowse(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this is a videoRenderer or richItemRenderer with videoId
		if videoID, ok := v["videoId"].(string); ok && videoID != "" {
			return videoID
		}

		// Recursively search all map values
		for _, val := range v {
			if videoID := extractVideoIDFromBrowse(val); videoID != "" {
				return videoID
			}
		}

	case []interface{}:
		// Recursively search all array elements
		for _, item := range v {
			if videoID := extractVideoIDFromBrowse(item); videoID != "" {
				return videoID
			}
		}
	}

	return ""
}

// GetInitialContinuation fetches the initial continuation token for a live stream
// Uses the InnerTube /next API to get the live chat continuation token for a video
func (d *Discovery) GetInitialContinuation(ctx context.Context, videoID string) (string, error) {
	d.logger.Info("fetching initial continuation token",
		zap.String("video_id", videoID),
	)

	// Use InnerTube /next API to get video metadata including live chat continuation
	url := fmt.Sprintf("https://www.youtube.com/youtubei/v1/next?key=%s", DefaultAPIKey)

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": DefaultClientVersion,
			},
		},
		"videoId": videoID,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Referer", fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch next API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code from next API: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	// Parse JSON response and extract continuation token recursively
	var nextResponse map[string]interface{}
	if err := json.Unmarshal(body, &nextResponse); err != nil {
		return "", fmt.Errorf("parse next API response: %w", err)
	}

	// Extract continuation token from the nested response structure
	// Live chat continuation is nested under engagementPanels → liveChatRenderer
	token := extractLiveChatContinuationFromNext(nextResponse)
	if token == "" {
		return "", fmt.Errorf("no live chat continuation token found in next API response for video %s", videoID)
	}

	d.logger.Info("extracted initial continuation token via next API",
		zap.String("video_id", videoID),
		zap.Int("token_length", len(token)),
	)

	return token, nil
}

// extractLiveChatContinuationFromNext extracts the live chat continuation token
// from the InnerTube /next API response. The token is nested inside engagementPanels
// under a liveChatRenderer with a continuationData.continuation field.
func extractLiveChatContinuationFromNext(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check for continuation inside liveChatRenderer
		if _, hasLiveChatRenderer := v["liveChatRenderer"]; hasLiveChatRenderer {
			if token := extractContinuationFromRenderer(v["liveChatRenderer"]); token != "" {
				return token
			}
		}

		// Recursively search all values
		for _, val := range v {
			if token := extractLiveChatContinuationFromNext(val); token != "" {
				return token
			}
		}

	case []interface{}:
		for _, item := range v {
			if token := extractLiveChatContinuationFromNext(item); token != "" {
				return token
			}
		}
	}

	return ""
}

// extractContinuationFromRenderer extracts the continuation token from a liveChatRenderer object
func extractContinuationFromRenderer(renderer interface{}) string {
	m, ok := renderer.(map[string]interface{})
	if !ok {
		return ""
	}

	// Look for header.liveChatHeaderRenderer.viewSelector.sortFilterSubMenuRenderer.subMenuItems
	// or continuations array directly
	continuations, ok := m["continuations"].([]interface{})
	if !ok {
		return ""
	}

	for _, cont := range continuations {
		contMap, ok := cont.(map[string]interface{})
		if !ok {
			continue
		}
		// Try reloadContinuationData first (most common for live streams)
		if reload, ok := contMap["reloadContinuationData"].(map[string]interface{}); ok {
			if token, ok := reload["continuation"].(string); ok && token != "" {
				return token
			}
		}
		// Try timedContinuationData
		if timed, ok := contMap["timedContinuationData"].(map[string]interface{}); ok {
			if token, ok := timed["continuation"].(string); ok && token != "" {
				return token
			}
		}
		// Try invalidationContinuationData
		if invalid, ok := contMap["invalidationContinuationData"].(map[string]interface{}); ok {
			if token, ok := invalid["continuation"].(string); ok && token != "" {
				return token
			}
		}
	}

	return ""
}
