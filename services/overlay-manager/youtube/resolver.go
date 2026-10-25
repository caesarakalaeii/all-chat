// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrQuotaExhausted is returned when YouTube API quota is exhausted
	ErrQuotaExhausted = errors.New("youtube API quota exhausted")
)

var (
	// YouTube URL patterns
	videoURLPattern   = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`)
	channelURLPattern = regexp.MustCompile(`youtube\.com/channel/([a-zA-Z0-9_-]+)`)
	handlePattern     = regexp.MustCompile(`youtube\.com/@([a-zA-Z0-9_.-]+)`)
	channelIDPattern  = regexp.MustCompile(`^UC[a-zA-Z0-9_-]{22}$`)
	videoIDPattern    = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
)

const (
	// defaultInnertubeAPIKey is the public InnerTube API key (same value used by
	// youtube-listener-innertube). Kept as the fallback for innertubeAPIKey.
	defaultInnertubeAPIKey = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"
	innertubeClientVersion = "2.20260312.01.00"
)

// innertubeAPIKey is the InnerTube API key used for resolve/browse calls.
// Configurable via the ALLCHAT_INNERTUBE_KEY env var (audit L26); falls back to
// the public web-client key when unset. Must match youtube-listener-innertube.
var innertubeAPIKey = getEnvDefault("ALLCHAT_INNERTUBE_KEY", defaultInnertubeAPIKey)

// getEnvDefault returns os.Getenv(key) when set and non-empty, else fallback.
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Resolver resolves YouTube URLs/handles to channel IDs using the InnerTube API.
// No YouTube Data API v3 quota is consumed.
type Resolver struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewResolver creates a new YouTube resolver.
// The apiKey and quotaClient parameters are accepted but ignored — resolution
// is performed via InnerTube which has no quota cost.
func NewResolver(_ string, _ interface{}, logger *zap.Logger) *Resolver {
	return &Resolver{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// ResolveToChannelID converts various YouTube inputs to a channel ID
func (r *Resolver) ResolveToChannelID(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)

	// Already a channel ID (starts with UC and is 24 chars)
	if channelIDPattern.MatchString(input) {
		return input, nil
	}

	// Extract from channel URL: youtube.com/channel/UC...
	if matches := channelURLPattern.FindStringSubmatch(input); len(matches) > 1 {
		return matches[1], nil
	}

	// Extract handle from full URL (youtube.com/@Handle) or bare @Handle
	var handle string
	if matches := handlePattern.FindStringSubmatch(input); len(matches) > 1 {
		handle = matches[1]
	} else if strings.HasPrefix(input, "@") {
		handle = strings.TrimPrefix(input, "@")
	}

	if handle != "" {
		return r.resolveHandleToChannelID(ctx, handle)
	}

	// Extract from video URL and look up channel via oEmbed
	if matches := videoURLPattern.FindStringSubmatch(input); len(matches) > 1 {
		return r.resolveVideoToChannelID(ctx, matches[1])
	}

	return "", fmt.Errorf("unable to parse YouTube input: %s", input)
}

// ParseVideoID extracts the 11-character video id from a pasted stream link: a bare id,
// youtube.com/watch?v=<id> (www./m./no scheme, any query order), youtu.be/<id> or
// youtube.com/live/<id>. The host is matched exactly, so lookalike and suffix hosts and
// channel, handle and playlist URLs are all rejected.
func ParseVideoID(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if videoIDPattern.MatchString(input) {
		return input, true
	}

	if !strings.Contains(input, "://") {
		input = "https://" + input
	}
	u, err := url.Parse(input)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", false
	}

	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	path := strings.TrimSuffix(u.Path, "/")

	var id string
	switch {
	case host == "youtu.be":
		id = strings.TrimPrefix(path, "/")
	case host == "youtube.com" && path == "/watch":
		id = u.Query().Get("v")
	case host == "youtube.com" && strings.HasPrefix(path, "/live/"):
		id = strings.TrimPrefix(path, "/live/")
	}
	if !videoIDPattern.MatchString(id) {
		return "", false
	}
	return id, true
}

// VideoChannelID returns the channel id that uploaded videoID. The watch page answers for
// unlisted and scheduled (upcoming) streams too, which is what lets a stream be pinned
// before it goes live.
func (r *Resolver) VideoChannelID(ctx context.Context, videoID string) (string, error) {
	return r.resolveVideoToChannelID(ctx, videoID)
}

// resolveHandleToChannelID resolves a YouTube @handle to a channel ID via the
// InnerTube navigation/resolve_url endpoint. No YouTube Data API v3 quota consumed.
func (r *Resolver) resolveHandleToChannelID(ctx context.Context, handle string) (string, error) {
	result, err := r.innertubePost(ctx, "navigation/resolve_url", map[string]interface{}{
		"url": "https://www.youtube.com/@" + handle,
	})
	if err != nil {
		return "", fmt.Errorf("resolve @%s: %w", handle, err)
	}

	// Response path: endpoint.browseEndpoint.browseId
	endpoint, _ := result["endpoint"].(map[string]interface{})
	browseEndpoint, _ := endpoint["browseEndpoint"].(map[string]interface{})
	channelID, _ := browseEndpoint["browseId"].(string)

	if channelID == "" || !channelIDPattern.MatchString(channelID) {
		return "", fmt.Errorf("no channel found for handle: @%s", handle)
	}
	return channelID, nil
}

// resolveVideoToChannelID resolves a video ID to its channel ID via the InnerTube /next
// (watch page) API. oEmbed was used before but omits author_url for scheduled streams.
func (r *Resolver) resolveVideoToChannelID(ctx context.Context, videoID string) (string, error) {
	data, err := r.innertubePost(ctx, "next", map[string]interface{}{"videoId": videoID})
	if err != nil {
		return "", fmt.Errorf("watch page for video %s: %w", videoID, err)
	}
	// Only the owner block counts: the rest of the page is recommendations from other channels.
	owner, _ := findKey(data, "videoOwnerRenderer").(map[string]interface{})
	nav, _ := owner["navigationEndpoint"].(map[string]interface{})
	browse, _ := nav["browseEndpoint"].(map[string]interface{})
	channelID, _ := browse["browseId"].(string)
	if !channelIDPattern.MatchString(channelID) {
		return "", fmt.Errorf("no owner channel on the watch page for video %s", videoID)
	}
	return channelID, nil
}

// findKey returns the value of the first key named key in a depth-first walk of v, or nil.
func findKey(v interface{}, key string) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if found, ok := t[key]; ok {
			return found
		}
		for _, child := range t {
			if found := findKey(child, key); found != nil {
				return found
			}
		}
	case []interface{}:
		for _, child := range t {
			if found := findKey(child, key); found != nil {
				return found
			}
		}
	}
	return nil
}

// GetChannelInfo returns display info for a channel ID via the InnerTube Browse API.
func (r *Resolver) GetChannelInfo(ctx context.Context, channelID string) (*ChannelInfo, error) {
	data, err := r.innertubePost(ctx, "browse", map[string]interface{}{"browseId": channelID})
	if err != nil {
		return nil, fmt.Errorf("innertube browse for channel %s: %w", channelID, err)
	}

	header := extractHeader(data)
	if header == nil {
		return nil, fmt.Errorf("no channel header in browse response for %s", channelID)
	}

	info := &ChannelInfo{ChannelID: channelID}

	if title, ok := header["title"].(string); ok {
		info.Title = title
	}
	if handle, ok := extractChannelHandle(header); ok {
		info.CustomURL = "@" + handle
	}
	if thumb := extractThumbnail(header); thumb != "" {
		info.Thumbnail = thumb
	}

	return info, nil
}

// innertubePost calls an InnerTube endpoint (browse, next, navigation/resolve_url) as the
// WEB client with fields added to the request body, and returns the decoded response.
func (r *Resolver) innertubePost(ctx context.Context, endpoint string, fields map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": innertubeClientVersion,
			},
		},
	}
	for k, v := range fields {
		payload[k] = v
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", endpoint, err)
	}

	endpointURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/%s?key=%s", endpoint, innertubeAPIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute %s request: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("innertube %s returned status %d", endpoint, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // audit #30: cap upstream body at 10 MiB
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", endpoint, err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse %s response: %w", endpoint, err)
	}
	return data, nil
}

// extractHeader returns the c4TabbedHeaderRenderer map from a browse response.
func extractHeader(data map[string]interface{}) map[string]interface{} {
	h, _ := data["header"].(map[string]interface{})
	if h == nil {
		return nil
	}
	renderer, _ := h["c4TabbedHeaderRenderer"].(map[string]interface{})
	return renderer
}

// extractChannelHandle tries to pull the handle from the header renderer.
func extractChannelHandle(header map[string]interface{}) (string, bool) {
	// channelHandleText.runs[0].text  →  "@handle"
	if cht, ok := header["channelHandleText"].(map[string]interface{}); ok {
		if runs, ok := cht["runs"].([]interface{}); ok && len(runs) > 0 {
			if run, ok := runs[0].(map[string]interface{}); ok {
				if text, ok := run["text"].(string); ok {
					return strings.TrimPrefix(text, "@"), true
				}
			}
		}
	}
	return "", false
}

// extractThumbnail returns the best available thumbnail URL from the header renderer.
func extractThumbnail(header map[string]interface{}) string {
	avatar, _ := header["avatar"].(map[string]interface{})
	if avatar == nil {
		return ""
	}
	thumbs, _ := avatar["thumbnails"].([]interface{})
	if len(thumbs) == 0 {
		return ""
	}
	// Prefer the last (highest resolution) thumbnail
	last := thumbs[len(thumbs)-1]
	if t, ok := last.(map[string]interface{}); ok {
		if u, ok := t["url"].(string); ok {
			return u
		}
	}
	return ""
}

// ChannelInfo contains YouTube channel information
type ChannelInfo struct {
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CustomURL   string `json:"custom_url"`
	Thumbnail   string `json:"thumbnail"`
}
