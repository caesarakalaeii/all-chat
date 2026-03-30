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
	httpClient    *http.Client
	logger        *zap.Logger
	apiKey        string
	clientVersion string
}

// NewDiscovery creates a new Discovery instance
func NewDiscovery(httpClient *http.Client, logger *zap.Logger, cfg ClientConfig) *Discovery {
	if cfg.APIKey == "" {
		cfg.APIKey = DefaultAPIKey
	}
	if cfg.ClientVersion == "" {
		cfg.ClientVersion = DefaultClientVersion
	}
	return &Discovery{
		httpClient:    httpClient,
		logger:        logger,
		apiKey:        cfg.APIKey,
		clientVersion: cfg.ClientVersion,
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
	browseURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/browse?key=%s", d.apiKey)
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": d.clientVersion,
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

	// Collect live video IDs directly from the browse response using the LIVE badge.
	// The player API (/youtubei/v1/player) is blocked by YouTube bot-detection on
	// datacenter IPs (returns LOGIN_REQUIRED with no videoDetails), so we rely on the
	// thumbnailOverlayTimeStatusRenderer.style == "LIVE" field present in the browse
	// response for each currently-live stream.
	videoIDs := collectLiveVideoIDsFromBrowse(browseResponse)
	if len(videoIDs) == 0 {
		return "", fmt.Errorf("no live stream found for channel %s", channelID)
	}

	videoID := videoIDs[0]
	d.logger.Info("discovered live stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)
	return videoID, nil
}

// checkIsLiveViaPlayer uses the InnerTube player API to check if a video is currently live.
// Returns true only when videoDetails.isLive is explicitly true.
// Works for restricted/members-only channels (returns isLive even when status=UNPLAYABLE).
func (d *Discovery) checkIsLiveViaPlayer(ctx context.Context, videoID string) (bool, error) {
	playerURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/player?key=%s", d.apiKey)
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": d.clientVersion,
			},
		},
		"videoId": videoID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var playerResp map[string]interface{}
	if err := json.Unmarshal(body, &playerResp); err != nil {
		return false, err
	}

	videoDetails, _ := playerResp["videoDetails"].(map[string]interface{})
	isLive, _ := videoDetails["isLive"].(bool)
	return isLive, nil
}

// collectLiveVideoIDsFromBrowse recursively finds video IDs that are currently live
// by checking for thumbnailOverlayTimeStatusRenderer.style == "LIVE" in the browse response.
// This avoids calling the player API which is blocked on datacenter IPs by YouTube bot-detection.
func collectLiveVideoIDsFromBrowse(data interface{}) []string {
	var ids []string
	seen := map[string]struct{}{}
	var collect func(interface{})
	collect = func(data interface{}) {
		if len(ids) >= 5 {
			return
		}
		switch v := data.(type) {
		case map[string]interface{}:
			// Check if this map is a videoRenderer with a LIVE overlay
			if videoID, ok := v["videoId"].(string); ok && videoID != "" {
				if overlays, ok := v["thumbnailOverlays"].([]interface{}); ok {
					for _, overlay := range overlays {
						ovMap, ok := overlay.(map[string]interface{})
						if !ok {
							continue
						}
						ts, ok := ovMap["thumbnailOverlayTimeStatusRenderer"].(map[string]interface{})
						if !ok {
							continue
						}
						if style, _ := ts["style"].(string); style == "LIVE" {
							if _, exists := seen[videoID]; !exists {
								seen[videoID] = struct{}{}
								ids = append(ids, videoID)
							}
							break
						}
					}
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
// Uses the InnerTube /next API (JSON, no HTML scraping, no rate limiting).
// The token is found at: contents.twoColumnWatchNextResults.conversationBar.liveChatRenderer
// Only returns a token when the stream is currently live (liveChatRenderer only present for live streams).
func (d *Discovery) GetInitialContinuation(ctx context.Context, videoID string) (string, string, error) {
	d.logger.Info("fetching initial continuation token",
		zap.String("video_id", videoID),
	)

	nextURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/next?key=%s", d.apiKey)
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": d.clientVersion,
			},
		},
		"videoId": videoID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nextURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch next API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code from next API: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response body: %w", err)
	}

	var nextData map[string]interface{}
	if err := json.Unmarshal(body, &nextData); err != nil {
		return "", "", fmt.Errorf("parse next API response: %w", err)
	}

	// Extract visitorData from responseContext
	var visitorData string
	if rc, ok := nextData["responseContext"].(map[string]interface{}); ok {
		if vd, ok := rc["visitorData"].(string); ok {
			visitorData = vd
		}
	}

	// Walk the known path to the liveChatRenderer
	token := extractContinuationFromNextAPI(nextData)
	if token == "" {
		return "", "", fmt.Errorf("no live chat continuation found in next API for video %s (stream may have ended)", videoID)
	}

	d.logger.Info("extracted initial continuation token from next API",
		zap.String("video_id", videoID),
		zap.Int("token_length", len(token)),
		zap.Bool("has_visitor_data", visitorData != ""),
	)

	return token, visitorData, nil
}

// extractContinuationFromNextAPI extracts the live chat continuation token from
// the InnerTube /next API response. For live streams, it's at:
// contents.twoColumnWatchNextResults.conversationBar.liveChatRenderer.continuations[].reloadContinuationData.continuation
func extractContinuationFromNextAPI(data map[string]interface{}) string {
	// Walk the known path
	contents, _ := data["contents"].(map[string]interface{})
	twoCol, _ := contents["twoColumnWatchNextResults"].(map[string]interface{})
	bar, _ := twoCol["conversationBar"].(map[string]interface{})
	renderer, _ := bar["liveChatRenderer"].(map[string]interface{})
	if renderer == nil {
		return ""
	}
	// isReplay=true means stream ended; its tokens are rejected by get_live_chat
	if isReplay, _ := renderer["isReplay"].(bool); isReplay {
		return ""
	}
	return extractContinuationFromLiveChatRenderer(renderer)
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
			// Last key should be the liveChatRenderer - skip if it's a replay
			rendererMap, _ := val.(map[string]interface{})
			if isReplay, _ := rendererMap["isReplay"].(bool); isReplay {
				return ""
			}
			return extractContinuationFromLiveChatRenderer(val)
		}
		current = val
	}
	return ""
}

// searchLiveChatRenderer recursively searches for liveChatRenderer in any map.
// Skips renderers with isReplay=true (chat replay for ended streams uses a different API).
func searchLiveChatRenderer(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		if renderer, ok := v["liveChatRenderer"]; ok {
			rendererMap, _ := renderer.(map[string]interface{})
			// Skip replay chat - its tokens are rejected by get_live_chat endpoint
			if isReplay, _ := rendererMap["isReplay"].(bool); isReplay {
				return ""
			}
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
// from a liveChatRenderer object.
//
// YouTube returns both a main continuations array and per-view subMenuItem
// tokens ("Live chat" / "Top chat"). The main continuations array token
// corresponds to whichever chat view was last active — which may be "Top chat"
// after an OBS restart or when the streamer's Studio had top-chat selected.
// To guarantee we always get "Live chat" (all messages), we prefer the
// explicit "Live chat" subMenuItem token when available, falling back to the
// main continuations array only when subMenuItems are absent (e.g. small
// streams without a view selector).
func extractContinuationFromLiveChatRenderer(renderer interface{}) string {
	m, ok := renderer.(map[string]interface{})
	if !ok {
		return ""
	}

	// Prefer the explicit "Live chat" subMenuItem token — this guarantees we
	// get all messages, not the filtered "Top chat" view.
	if token := extractLiveChatSubMenuToken(m); token != "" {
		return token
	}

	// Fallback: main continuations array. This may correspond to either chat
	// view, but is the only option when subMenuItems are absent.
	continuations, ok := m["continuations"].([]interface{})
	if ok {
		for _, cont := range continuations {
			contMap, ok := cont.(map[string]interface{})
			if !ok {
				continue
			}
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
	}

	return ""
}

// extractLiveChatSubMenuToken walks
// liveChatRenderer.header.liveChatHeaderRenderer.viewSelector
//   .sortFilterSubMenuRenderer.subMenuItems
// and returns the continuation token for the "Live chat" item (all messages).
// Returns "" if the structure is absent or no "Live chat" item is found.
func extractLiveChatSubMenuToken(renderer map[string]interface{}) string {
	header, _ := renderer["header"].(map[string]interface{})
	lch, _ := header["liveChatHeaderRenderer"].(map[string]interface{})
	vs, _ := lch["viewSelector"].(map[string]interface{})
	sfr, _ := vs["sortFilterSubMenuRenderer"].(map[string]interface{})
	items, _ := sfr["subMenuItems"].([]interface{})

	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if item["title"] != "Live chat" {
			continue
		}
		cont, _ := item["continuation"].(map[string]interface{})
		reload, _ := cont["reloadContinuationData"].(map[string]interface{})
		if token, ok := reload["continuation"].(string); ok && token != "" {
			return token
		}
	}
	return ""
}
