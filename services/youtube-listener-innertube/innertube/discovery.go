package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

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

// DiscoverLiveStream discovers the live video ID for a given channel ID.
// Uses the InnerTube Browse API to list recent streams, then verifies liveness
// via the player API to avoid returning ended/upcoming streams.
func (d *Discovery) DiscoverLiveStream(ctx context.Context, channelID string) (string, error) {
	d.logger.Info("discovering live stream",
		zap.String("channel_id", channelID),
	)

	// Use InnerTube Browse API to get channel's streams tab
	browseURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/browse?key=%s", DefaultAPIKey)
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": DefaultClientVersion,
			},
		},
		"browseId": channelID,
		"params":   "EgdzdHJlYW1z8gYECgJ6AA%3D%3D", // "streams" tab
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, browseURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch channel browse data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	var browseResponse map[string]interface{}
	if err := json.Unmarshal(body, &browseResponse); err != nil {
		return "", fmt.Errorf("parse browse response: %w", err)
	}

	// Collect all video IDs from the streams tab (may include ended streams)
	videoIDs := collectVideoIDsFromBrowse(browseResponse)
	if len(videoIDs) == 0 {
		return "", fmt.Errorf("no streams found for channel %s", channelID)
	}

	// Verify liveness for each video by checking the watch page for a liveChatRenderer.
	// The player API is unreliable for restricted/members-only channels (returns UNPLAYABLE).
	// The presence of liveChatRenderer in the watch page's ytInitialData is the definitive
	// indicator that a stream is currently live and has an active chat.
	for _, videoID := range videoIDs {
		token, err := d.GetInitialContinuation(ctx, videoID)
		if err != nil {
			d.logger.Debug("video not live (no live chat found)",
				zap.String("video_id", videoID),
				zap.Error(err),
			)
			continue
		}
		d.logger.Info("discovered live stream",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
		)
		// Return both videoID and token so startPoller doesn't need to re-fetch
		// Store the token in cache for the manager to use
		_ = token // token will be re-fetched by startPoller; discovery just confirms liveness
		return videoID, nil
	}

	return "", fmt.Errorf("no live stream found for channel %s", channelID)
}

// collectVideoIDsFromBrowse recursively collects all unique video IDs from a browse response.
// Returns up to 5 IDs to check for liveness (avoiding deep recursion on large responses).
func collectVideoIDsFromBrowse(data interface{}) []string {
	seen := map[string]struct{}{}
	var ids []string
	var collect func(interface{})
	collect = func(data interface{}) {
		if len(ids) >= 5 {
			return
		}
		switch v := data.(type) {
		case map[string]interface{}:
			if videoID, ok := v["videoId"].(string); ok && videoID != "" {
				if _, exists := seen[videoID]; !exists {
					seen[videoID] = struct{}{}
					ids = append(ids, videoID)
				}
			}
			for _, val := range v {
				collect(val)
			}
		case []interface{}:
			for _, item := range v {
				collect(item)
			}
		}
	}
	collect(data)
	return ids
}

// GetInitialContinuation fetches the initial continuation token for a live stream.
// Scrapes the watch page HTML to extract the liveChatRenderer continuation token,
// which is the correct token for use with the get_live_chat InnerTube endpoint.
func (d *Discovery) GetInitialContinuation(ctx context.Context, videoID string) (string, error) {
	d.logger.Info("fetching initial continuation token",
		zap.String("video_id", videoID),
	)

	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", "CONSENT=YES+cb; PREF=hl=en&gl=US")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch watch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	// Extract the ytInitialData JSON embedded in the page script
	initDataRe := regexp.MustCompile(`(?:var\s+)?ytInitialData\s*=\s*(\{.+?\});\s*(?:var\s|</script>)`)
	match := initDataRe.FindSubmatch(body)
	if match == nil {
		return "", fmt.Errorf("ytInitialData not found in watch page for video %s", videoID)
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(match[1], &pageData); err != nil {
		return "", fmt.Errorf("parse ytInitialData: %w", err)
	}

	// The live chat continuation is inside conversationBar.liveChatRenderer.continuations
	token := extractLiveChatContinuation(pageData)
	if token == "" {
		return "", fmt.Errorf("no live chat continuation found in watch page for video %s (stream may have ended)", videoID)
	}

	d.logger.Info("extracted initial continuation token from watch page",
		zap.String("video_id", videoID),
		zap.Int("token_length", len(token)),
	)

	return token, nil
}

// extractLiveChatContinuation finds the live chat continuation token inside ytInitialData.
// It's located at: contents.twoColumnWatchNextResults.conversationBar.liveChatRenderer.continuations
func extractLiveChatContinuation(data map[string]interface{}) string {
	// Walk the known path first
	if token := walkPath(data,
		"contents", "twoColumnWatchNextResults", "conversationBar",
		"liveChatRenderer",
	); token != "" {
		return token
	}
	// Fall back to recursive search for liveChatRenderer anywhere in the page
	return searchLiveChatRenderer(data)
}

// walkPath navigates a nested map following the given keys, then extracts
// the continuation token from a liveChatRenderer at the final key.
func walkPath(data map[string]interface{}, keys ...string) string {
	current := interface{}(data)
	for i, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		val, ok := m[key]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			// Last key should be the liveChatRenderer
			return extractContinuationFromLiveChatRenderer(val)
		}
		current = val
	}
	return ""
}

// searchLiveChatRenderer recursively searches for liveChatRenderer in any map
func searchLiveChatRenderer(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		if renderer, ok := v["liveChatRenderer"]; ok {
			if token := extractContinuationFromLiveChatRenderer(renderer); token != "" {
				return token
			}
		}
		for _, val := range v {
			if token := searchLiveChatRenderer(val); token != "" {
				return token
			}
		}
	case []interface{}:
		for _, item := range v {
			if token := searchLiveChatRenderer(item); token != "" {
				return token
			}
		}
	}
	return ""
}

// extractContinuationFromLiveChatRenderer extracts the continuation token
// from a liveChatRenderer object's continuations array.
func extractContinuationFromLiveChatRenderer(renderer interface{}) string {
	m, ok := renderer.(map[string]interface{})
	if !ok {
		return ""
	}
	continuations, ok := m["continuations"].([]interface{})
	if !ok {
		return ""
	}
	for _, cont := range continuations {
		contMap, ok := cont.(map[string]interface{})
		if !ok {
			continue
		}
		// reloadContinuationData is the standard for initial live chat fetch
		if reload, ok := contMap["reloadContinuationData"].(map[string]interface{}); ok {
			if token, ok := reload["continuation"].(string); ok && token != "" {
				return token
			}
		}
		if timed, ok := contMap["timedContinuationData"].(map[string]interface{}); ok {
			if token, ok := timed["continuation"].(string); ok && token != "" {
				return token
			}
		}
		if inv, ok := contMap["invalidationContinuationData"].(map[string]interface{}); ok {
			if token, ok := inv["continuation"].(string); ok && token != "" {
				return token
			}
		}
	}
	return ""
}
