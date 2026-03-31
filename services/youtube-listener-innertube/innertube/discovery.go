package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// Stream selection strategy constants
const (
	StrategyFirstFound    = "first_found"
	StrategyMostViewers   = "most_viewers"
	StrategyFewestViewers = "fewest_viewers"
	StrategyTitleMatch    = "title_match"
	StrategyAll           = "all"
)

// ValidStrategies contains all recognized stream selection strategies.
var ValidStrategies = map[string]bool{
	StrategyFirstFound:    true,
	StrategyMostViewers:   true,
	StrategyFewestViewers: true,
	StrategyTitleMatch:    true,
	StrategyAll:           true,
}

// LiveStreamCandidate represents a discovered live stream with metadata
// used for selection when a channel has multiple concurrent streams.
type LiveStreamCandidate struct {
	VideoID     string
	Title       string
	ViewerCount int // -1 if unknown
}

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
// Uses the InnerTube Browse API to list recent streams, then applies the
// given selection strategy to choose among multiple concurrent streams.
// strategy defaults to "first_found" if empty. matchTerm is only used
// with "title_match" strategy.
func (d *Discovery) DiscoverLiveStream(ctx context.Context, channelID, strategy, matchTerm string) (string, error) {
	if strategy == "" {
		strategy = StrategyFirstFound
	}

	d.logger.Info("discovering live stream",
		zap.String("channel_id", channelID),
		zap.String("strategy", strategy),
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

	// Collect live stream candidates from the browse response using the LIVE badge.
	// The player API (/youtubei/v1/player) is blocked by YouTube bot-detection on
	// datacenter IPs (returns LOGIN_REQUIRED with no videoDetails), so we rely on the
	// thumbnailOverlayTimeStatusRenderer.style == "LIVE" field present in the browse
	// response for each currently-live stream.
	candidates := collectLiveCandidatesFromBrowse(browseResponse)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no live stream found for channel %s", channelID)
	}

	selected, err := SelectStream(candidates, strategy, matchTerm)
	if err != nil {
		return "", fmt.Errorf("stream selection failed for channel %s: %w", channelID, err)
	}

	d.logger.Info("discovered live stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", selected.VideoID),
		zap.String("title", selected.Title),
		zap.Int("viewer_count", selected.ViewerCount),
		zap.Int("candidates", len(candidates)),
		zap.String("strategy", strategy),
	)
	return selected.VideoID, nil
}

// DiscoverAllLiveStreams discovers all live video IDs for a given channel.
// Used by the "all" strategy to start a poller for every concurrent stream.
func (d *Discovery) DiscoverAllLiveStreams(ctx context.Context, channelID string) ([]string, error) {
	d.logger.Info("discovering all live streams",
		zap.String("channel_id", channelID),
	)

	browseURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/browse?key=%s", d.apiKey)
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": d.clientVersion,
			},
		},
		"browseId": channelID,
		"params":   "EgdzdHJlYW1z8gYECgJ6AA%3D%3D",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, browseURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch channel browse data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var browseResponse map[string]interface{}
	if err := json.Unmarshal(body, &browseResponse); err != nil {
		return nil, fmt.Errorf("parse browse response: %w", err)
	}

	candidates := collectLiveCandidatesFromBrowse(browseResponse)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no live streams found for channel %s", channelID)
	}

	videoIDs := make([]string, len(candidates))
	for i, c := range candidates {
		videoIDs[i] = c.VideoID
	}

	d.logger.Info("discovered all live streams",
		zap.String("channel_id", channelID),
		zap.Int("count", len(videoIDs)),
		zap.Strings("video_ids", videoIDs),
	)
	return videoIDs, nil
}

// SelectStream applies the given strategy to choose a stream from candidates.
func SelectStream(candidates []LiveStreamCandidate, strategy, matchTerm string) (LiveStreamCandidate, error) {
	if len(candidates) == 0 {
		return LiveStreamCandidate{}, fmt.Errorf("no candidates")
	}

	switch strategy {
	case StrategyFirstFound, "":
		return candidates[0], nil

	case StrategyMostViewers:
		best := candidates[0]
		for _, c := range candidates[1:] {
			if c.ViewerCount > best.ViewerCount {
				best = c
			}
		}
		return best, nil

	case StrategyFewestViewers:
		best := candidates[0]
		for _, c := range candidates[1:] {
			// Prefer known viewer counts over unknown (-1)
			if best.ViewerCount < 0 || (c.ViewerCount >= 0 && c.ViewerCount < best.ViewerCount) {
				best = c
			}
		}
		return best, nil

	case StrategyTitleMatch:
		if matchTerm == "" {
			return candidates[0], nil
		}
		lower := strings.ToLower(matchTerm)
		for _, c := range candidates {
			if strings.Contains(strings.ToLower(c.Title), lower) {
				return c, nil
			}
		}
		// No title match found — fall back to first
		return candidates[0], nil

	default:
		return candidates[0], nil
	}
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

// collectLiveCandidatesFromBrowse recursively finds live stream candidates
// by checking for thumbnailOverlayTimeStatusRenderer.style == "LIVE" in the browse response.
// Extracts videoId, title, and viewer count for each live stream to support
// stream selection strategies (most viewers, title match, etc.).
func collectLiveCandidatesFromBrowse(data interface{}) []LiveStreamCandidate {
	var candidates []LiveStreamCandidate
	seen := map[string]struct{}{}
	var collect func(interface{})
	collect = func(data interface{}) {
		if len(candidates) >= 5 {
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
								candidates = append(candidates, LiveStreamCandidate{
									VideoID:     videoID,
									Title:       extractTitle(v),
									ViewerCount: extractViewerCount(v),
								})
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
	return candidates
}

// collectLiveVideoIDsFromBrowse is a convenience wrapper that returns only video IDs.
// Kept for backward compatibility with callers that don't need full candidate metadata.
func collectLiveVideoIDsFromBrowse(data interface{}) []string {
	candidates := collectLiveCandidatesFromBrowse(data)
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.VideoID
	}
	return ids
}

// extractTitle extracts the stream title from a videoRenderer map.
// YouTube stores titles as runs: {"title": {"runs": [{"text": "..."}]}}.
func extractTitle(renderer map[string]interface{}) string {
	title, ok := renderer["title"].(map[string]interface{})
	if !ok {
		return ""
	}
	runs, ok := title["runs"].([]interface{})
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, run := range runs {
		if runMap, ok := run.(map[string]interface{}); ok {
			if text, ok := runMap["text"].(string); ok {
				sb.WriteString(text)
			}
		}
	}
	return sb.String()
}

// viewerCountRegex matches numbers in "1,234 watching now" or "1.234 watching" style text.
var viewerCountRegex = regexp.MustCompile(`[\d,.]+`)

// extractViewerCount extracts the viewer count from a videoRenderer map.
// YouTube provides this as viewCountText: {"simpleText": "1,234 watching now"}
// or as runs: [{"text": "1,234"}, {"text": " watching now"}].
// Returns -1 if the count cannot be determined.
func extractViewerCount(renderer map[string]interface{}) int {
	vct, ok := renderer["viewCountText"].(map[string]interface{})
	if !ok {
		return -1
	}

	var raw string

	// Try simpleText first
	if st, ok := vct["simpleText"].(string); ok {
		raw = st
	} else if runs, ok := vct["runs"].([]interface{}); ok && len(runs) > 0 {
		// Take text from first run (the number part)
		if runMap, ok := runs[0].(map[string]interface{}); ok {
			if text, ok := runMap["text"].(string); ok {
				raw = text
			}
		}
	}

	if raw == "" {
		return -1
	}

	// Extract numeric portion and strip thousands separators
	match := viewerCountRegex.FindString(raw)
	if match == "" {
		return -1
	}
	cleaned := strings.NewReplacer(",", "", ".", "").Replace(match)
	count, err := strconv.Atoi(cleaned)
	if err != nil {
		return -1
	}
	return count
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
// Uses the InnerTube /next API to verify the stream is live and obtain visitorData,
// then generates the continuation token from scratch via protobuf encoding to
// guarantee "Live chat" (all messages) rather than "Top chat" (filtered).
func (d *Discovery) GetInitialContinuation(ctx context.Context, videoID, channelID string) (string, string, error) {
	d.logger.Info("fetching initial continuation token",
		zap.String("video_id", videoID),
		zap.String("channel_id", channelID),
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

	// Verify the stream is live by checking for liveChatRenderer presence.
	// We don't use the extracted token — we generate our own.
	if extractContinuationFromNextAPI(nextData) == "" {
		return "", "", fmt.Errorf("no live chat continuation found in next API for video %s (stream may have ended)", videoID)
	}

	// Generate continuation token from scratch with chattype=1 (all messages).
	// This avoids depending on YouTube's subMenuItem tokens which are rejected
	// with HTTP 400 and the main continuations array which defaults to Top Chat.
	token := GenerateLiveChatContinuation(videoID, channelID, ChatTypeAll)

	d.logger.Info("initial continuation token ready",
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

// extractContinuationFromLiveChatRenderer extracts a continuation token from
// a liveChatRenderer object. Used only for liveness verification — the actual
// polling token is generated from scratch via GenerateLiveChatContinuation.
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

