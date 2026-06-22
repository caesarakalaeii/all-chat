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

package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// StreamState mirrors the canonical youtube:stream:state:<channelID> contract originally
// written by the quota-based youtube-listener (services/youtube-listener/streams/
// stream_state_store.go) and read by auth-service (streamer chat send) and
// moderation-service (cross-platform mod actions) to resolve a channel's official live
// chat. The deployed InnerTube listener polls chat via continuation tokens and never sees
// the official activeLiveChatId, so without this the consumers' fast-path cache is always
// empty and they fall back to the unreliable search.list API. Consumers only require
// live_chat_id + is_live; the remaining fields are kept for schema parity. See ADR-0024.
type StreamState struct {
	ChannelID   string    `json:"channel_id"`
	StreamID    string    `json:"stream_id"`
	LiveChatID  string    `json:"live_chat_id"`
	OverlayID   string    `json:"overlay_id"`
	IsLive      bool      `json:"is_live"`
	LastUpdated time.Time `json:"last_updated"`
}

// LiveChatResolver resolves a live video's official activeLiveChatId. A nil resolver
// (no API key configured) disables live-chat-id caching — the listener still reads chat,
// streamer sends just fall back to their existing discovery path.
type LiveChatResolver interface {
	Resolve(ctx context.Context, videoID string) (string, error)
}

// DataAPILiveChatResolver resolves activeLiveChatId via the YouTube Data API videos.list
// endpoint (part=liveStreamingDetails). This costs 1 quota unit and is reliable for active
// live streams — unlike the search.list discovery path that returns accountDelegationForbidden.
// One call per stream start (not per poll), so quota impact is negligible.
type DataAPILiveChatResolver struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string // overridable in tests; defaults to the Data API videos endpoint
	logger     *zap.Logger
}

// NewDataAPILiveChatResolver builds a resolver bound to the given Data API key.
func NewDataAPILiveChatResolver(apiKey string, logger *zap.Logger) *DataAPILiveChatResolver {
	return &DataAPILiveChatResolver{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		baseURL:    "https://www.googleapis.com/youtube/v3/videos",
		logger:     logger,
	}
}

// Resolve returns the activeLiveChatId for videoID, or an error if the video has no active
// live chat (not live / chat disabled) or the API call fails.
func (r *DataAPILiveChatResolver) Resolve(ctx context.Context, videoID string) (string, error) {
	u := fmt.Sprintf("%s?part=liveStreamingDetails&id=%s&key=%s",
		r.baseURL, url.QueryEscape(videoID), url.QueryEscape(r.apiKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("create videos.list request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("videos.list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("videos.list status=%d body=%s", resp.StatusCode, string(body))
	}

	var out struct {
		Items []struct {
			LiveStreamingDetails struct {
				ActiveLiveChatID string `json:"activeLiveChatId"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode videos.list response: %w", err)
	}

	if len(out.Items) == 0 || out.Items[0].LiveStreamingDetails.ActiveLiveChatID == "" {
		return "", fmt.Errorf("no active live chat for video %s", videoID)
	}
	return out.Items[0].LiveStreamingDetails.ActiveLiveChatID, nil
}
