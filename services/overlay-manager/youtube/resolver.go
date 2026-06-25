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

// resolveHandleToChannelID resolves a YouTube @handle to a channel ID via the
// InnerTube navigation/resolve_url endpoint. No YouTube Data API v3 quota consumed.
func (r *Resolver) resolveHandleToChannelID(ctx context.Context, handle string) (string, error) {
	channelURL := "https://www.youtube.com/@" + handle

	resolveURL := fmt.Sprintf("https://www.youtube.com/youtubei/v1/navigation/resolve_url?key=%s", innertubeAPIKey)
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": innertubeClientVersion,
			},
		},
		"url": channelURL,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal resolve_url payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolveURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("create resolve_url request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute resolve_url request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve_url returned status %d for @%s", resp.StatusCode, handle)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // audit #30: cap upstream body at 10 MiB
	if err != nil {
		return "", fmt.Errorf("read resolve_url response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse resolve_url response: %w", err)
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

// resolveVideoToChannelID resolves a video ID to its channel ID via YouTube oEmbed.
// oEmbed returns the author_url which contains the channel URL.
func (r *Resolver) resolveVideoToChannelID(ctx context.Context, videoID string) (string, error) {
	oembedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json", videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return "", fmt.Errorf("create oembed request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch oembed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oembed returned status %d for video %s", resp.StatusCode, videoID)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // audit #30: cap upstream body at 10 MiB
	if err != nil {
		return "", fmt.Errorf("read oembed response: %w", err)
	}

	var oembed struct {
		AuthorURL string `json:"author_url"`
	}
	if err := json.Unmarshal(body, &oembed); err != nil {
		return "", fmt.Errorf("parse oembed response: %w", err)
	}

	// author_url is like https://www.youtube.com/channel/UC... or https://www.youtube.com/@handle
	if matches := channelURLPattern.FindStringSubmatch(oembed.AuthorURL); len(matches) > 1 {
		return matches[1], nil
	}
	if matches := handlePattern.FindStringSubmatch(oembed.AuthorURL); len(matches) > 1 {
		return r.resolveHandleToChannelID(ctx, matches[1])
	}

	return "", fmt.Errorf("could not extract channel ID from oembed author_url: %s", oembed.AuthorURL)
}

// GetChannelInfo returns display info for a channel ID via the InnerTube Browse API.
func (r *Resolver) GetChannelInfo(ctx context.Context, channelID string) (*ChannelInfo, error) {
	data, err := r.innertubeBrowse(ctx, channelID)
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

// innertubeBrowse calls the InnerTube Browse API for the given browseId.
func (r *Resolver) innertubeBrowse(ctx context.Context, browseID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://www.youtube.com/youtubei/v1/browse?key=%s", innertubeAPIKey)

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": innertubeClientVersion,
			},
		},
		"browseId": browseID,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("innertube browse returned status %d for %s", resp.StatusCode, browseID)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // audit #30: cap upstream body at 10 MiB
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	r.logger.Debug("innertube browse response",
		zap.String("browse_id", browseID),
		zap.Int("body_bytes", len(body)),
	)

	return data, nil
}

// extractChannelIDFromBrowse extracts the channel ID from an InnerTube browse response.
// The channel ID lives in header.c4TabbedHeaderRenderer.channelId.
func extractChannelIDFromBrowse(data map[string]interface{}) string {
	header := extractHeader(data)
	if header == nil {
		return ""
	}
	if id, ok := header["channelId"].(string); ok && id != "" {
		return id
	}
	return ""
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
